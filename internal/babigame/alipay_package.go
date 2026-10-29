package babigame

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

const alipayPackageLimit = 16 << 20
const alipayManifestLimit = 4 << 10

var alipayPackagePath = regexp.MustCompile(`^/gamecenteruprod_pkg/afts/file/[A-Za-z0-9_-]+$`)
var alipayExecutableVersion = regexp.MustCompile(`^[0-9]{1,6}\.[0-9]{1,6}\.[0-9]{1,6}$`)

func validAlipayPackageURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "mdn.alipayobjects.com" &&
		u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" &&
		u.RawPath == "" && alipayPackagePath.MatchString(u.Path)
}

// queryPackageVersion reads only Manifest.xml from the official package selected
// by queryPcGameInfo. Do not execute or unpack game code, guess versions, reuse
// an obsolete fallback, or substitute the SDK entry version from pack/init.
func (c *AlipayClient) queryPackageVersion(ctx context.Context, rawURL string) (string, error) {
	if !validAlipayPackageURL(rawURL) {
		return "", errors.New("alipay package URL is not an official game package")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", errors.New("alipay package request invalid")
	}
	// Never forward the channel grant or cookies to the asset host, nor follow
	// a redirect out of the pinned official package location.
	client := *c.httpClient()
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("alipay package download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("alipay package download HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > alipayPackageLimit {
		return "", errors.New("alipay package exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, alipayPackageLimit+1))
	if err != nil {
		return "", fmt.Errorf("alipay package read: %w", err)
	}
	if len(data) > alipayPackageLimit {
		return "", errors.New("alipay package exceeds size limit")
	}
	return alipayVersionFromPackage(data)
}

func alipayVersionFromPackage(data []byte) (string, error) {
	pkg, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("alipay package is not a valid ZIP archive")
	}
	var manifest *zip.File
	for _, file := range pkg.File {
		if file.Name != "Manifest.xml" {
			continue
		}
		if manifest != nil {
			return "", errors.New("alipay package has ambiguous manifests")
		}
		manifest = file
	}
	if manifest == nil || manifest.UncompressedSize64 > alipayManifestLimit {
		return "", errors.New("alipay package manifest missing or too large")
	}
	f, err := manifest.Open()
	if err != nil {
		return "", errors.New("alipay package manifest cannot be opened")
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, alipayManifestLimit+1))
	if err != nil || len(raw) > alipayManifestLimit {
		return "", errors.New("alipay package manifest cannot be read")
	}
	var info struct {
		XMLName  xml.Name `xml:"package"`
		AppIDs   []string `xml:"uid"`
		Versions []string `xml:"appVersion"`
	}
	if err := xml.Unmarshal(raw, &info); err != nil || len(info.AppIDs) != 1 || info.AppIDs[0] != alipayMiniGameAppID ||
		len(info.Versions) != 1 || !alipayExecutableVersion.MatchString(info.Versions[0]) {
		return "", errors.New("alipay package manifest has invalid game identity or executable version")
	}
	return info.Versions[0], nil
}
