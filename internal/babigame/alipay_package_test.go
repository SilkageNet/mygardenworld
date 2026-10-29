package babigame

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

const alipayPackageFixtureURL = "https://mdn.alipayobjects.com/gamecenteruprod_pkg/afts/file/test-package"
const alipayGameInfoFixture = `{"success":true,"data":{"appId":"2021004163668677","pkgUrl":{"pkgUrl":"` + alipayPackageFixtureURL + `"}}}`

func alipayManifestFixture(version string) string {
	return `<package><uid>2021004163668677</uid><version>0.2.0</version><appVersion>` + version + `</appVersion></package>`
}

func testAlipayPackage(t *testing.T, manifests ...string) []byte {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for _, manifest := range manifests {
		f, err := w.Create("Manifest.xml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, manifest); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func alipayPackageResponse(data []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}
}

func TestAlipayPackageManifest(t *testing.T) {
	valid := alipayManifestFixture("490.0.5")
	for _, tc := range []struct {
		name      string
		manifests []string
		want      string
	}{
		{"current executable", []string{valid}, "490.0.5"},
		{"future executable", []string{alipayManifestFixture("501.2.3")}, "501.2.3"},
		{"missing manifest", nil, ""},
		{"duplicate manifests", []string{valid, valid}, ""},
		{"wrong app", []string{strings.ReplaceAll(valid, alipayMiniGameAppID, "other")}, ""},
		{"duplicate app", []string{strings.ReplaceAll(valid, "</uid>", "</uid><uid>other</uid>")}, ""},
		{"duplicate version", []string{strings.ReplaceAll(valid, "</package>", "<appVersion>1.2.3</appVersion></package>")}, ""},
		{"package version is not executable", []string{`<package><uid>2021004163668677</uid><version>490.0.5</version></package>`}, ""},
		{"invalid version", []string{alipayManifestFixture("../prod?token=secret")}, ""},
		{"invalid XML", []string{`<package><uid>`}, ""},
		{"large manifest", []string{strings.Repeat("x", alipayManifestLimit+1)}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := alipayVersionFromPackage(testAlipayPackage(t, tc.manifests...))
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("version=%q err=%v", got, err)
			}
		})
	}
	if _, err := alipayVersionFromPackage([]byte("not a package")); err == nil {
		t.Fatal("invalid ZIP accepted")
	}
}

func TestAlipayPackageDownloadBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint     string
		status             int
		large, unknownSize bool
	}{
		{"official", alipayPackageFixtureURL, 200, false, false},
		{"HTTP forbidden", strings.Replace(alipayPackageFixtureURL, "https:", "http:", 1), 0, false, false},
		{"wrong host", strings.Replace(alipayPackageFixtureURL, "mdn.alipayobjects.com", "127.0.0.1", 1), 0, false, false},
		{"wrong path", "https://mdn.alipayobjects.com/other/file", 0, false, false},
		{"embedded credential", "https://secret@mdn.alipayobjects.com/gamecenteruprod_pkg/afts/file/test", 0, false, false},
		{"query", alipayPackageFixtureURL + "?token=secret", 0, false, false},
		{"redirect", alipayPackageFixtureURL, 302, false, false},
		{"unavailable", alipayPackageFixtureURL, 503, false, false},
		{"oversized", alipayPackageFixtureURL, 200, true, false},
		{"oversized stream", alipayPackageFixtureURL, 200, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelAlipay)
			c := NewAlipayClient(cfg)
			jar, _ := cookiejar.New(nil)
			u, _ := url.Parse(alipayPackageFixtureURL)
			jar.SetCookies(u, []*http.Cookie{{Name: "credential", Value: "private"}})
			c.HTTPClient.Jar = jar
			calls := 0
			c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 || req.Header.Get("Cookie") != "" || req.Header.Get(alipayWebTokenHeader) != "" {
					t.Fatal("asset fetch followed redirect or sent credentials")
				}
				data := testAlipayPackage(t, alipayManifestFixture("490.0.5"))
				if tc.large {
					data = make([]byte, alipayPackageLimit+1)
				}
				resp := alipayPackageResponse(data)
				resp.StatusCode = tc.status
				resp.Header.Set("Location", "http://127.0.0.1/private")
				if tc.unknownSize {
					resp.ContentLength = -1
				}
				return resp, nil
			})
			got, err := c.queryPackageVersion(context.Background(), tc.endpoint)
			if (err == nil) != (tc.name == "official") || (err == nil && got != "490.0.5") {
				t.Fatalf("version=%q err=%v", got, err)
			}
			if tc.status == 0 && calls != 0 {
				t.Fatal("invalid URL sent a request")
			}
		})
	}
}

func TestAlipayInvalidPackageStopsBeforeAuthorization(t *testing.T) {
	for _, tc := range []struct{ name, info, manifest string }{
		{"wrong app", strings.ReplaceAll(alipayGameInfoFixture, alipayMiniGameAppID, "other"), ""},
		{"missing package", `{"success":true,"data":{"appId":"2021004163668677"}}`, ""},
		{"invalid manifest", alipayGameInfoFixture, alipayManifestFixture("invalid")},
		{"audit route", alipayGameInfoFixture, alipayManifestFixture("490.0.5")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelAlipay)
			game := NewHTTPClient(cfg, "device", "uuid", "session0")
			provider := NewAlipayClient(cfg)
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "/queryPcGameInfo/") {
					return jsonResponse(tc.info), nil
				}
				if req.URL.String() == alipayPackageFixtureURL {
					return alipayPackageResponse(testAlipayPackage(t, tc.manifest)), nil
				}
				if tc.name == "audit route" && strings.HasPrefix(req.URL.Path, "/pack/init/") {
					return jsonResponse(`{"status":"success","url":"https://hygncdn.babigame.cn/index-gn-zfb-.html?uuid=launch&env=aud-zfb&mdgid=163&mdclid=538&packageId=520"}`), nil
				}
				t.Fatalf("invalid package continued to %s", req.URL.Path)
				return nil, nil
			})
			game.HTTPClient.Transport, provider.HTTPClient.Transport = transport, transport
			if _, err := provider.LoginWithWebGrant(context.Background(), game, AlipayWebGrant{Token: "token", UserID: "user"}); err == nil {
				t.Fatal("invalid official package accepted")
			}
			if game.Cfg != cfg {
				t.Fatal("failed metadata mutated channel config")
			}
		})
	}
}
