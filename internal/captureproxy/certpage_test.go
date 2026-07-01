package captureproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeCertificatePage(t *testing.T) {
	p := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8888/", nil)
	rec := httptest.NewRecorder()
	if !p.serveManagement(rec, req) {
		t.Fatal("serveManagement returned false")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"mygardenworld Capture CA", "/ca.crt", "/ca.cer", "SHA-256"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q: %s", want, body)
		}
	}
}

func TestServeCertificateDownload(t *testing.T) {
	p := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8888/ca.crt", nil)
	rec := httptest.NewRecorder()
	if !p.serveManagement(rec, req) {
		t.Fatal("serveManagement returned false")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Equal(rec.Body.Bytes(), p.ca.CertPEM) {
		t.Fatal("download body does not match CA PEM")
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "mygardenworld-capture-ca.crt") {
		t.Fatalf("Content-Disposition=%q", got)
	}
}

func TestServeCertificatePageThroughProxyForm(t *testing.T) {
	p := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8888/ca.cer", nil)
	req.URL.Scheme = "http"
	req.URL.Host = "192.0.2.10:8888"
	rec := httptest.NewRecorder()
	if !p.serveManagement(rec, req) {
		t.Fatal("serveManagement returned false for proxy-form cert URL")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Equal(rec.Body.Bytes(), p.ca.Cert.Certificate[0]) {
		t.Fatal("download body does not match CA DER")
	}
}

func TestServeManagementIgnoresUnrelatedAbsoluteURL(t *testing.T) {
	p := newTestProxy(t)

	req := httptest.NewRequest(http.MethodGet, "http://example.com:80/ca.crt", nil)
	rec := httptest.NewRecorder()
	if p.serveManagement(rec, req) {
		t.Fatal("serveManagement handled an unrelated absolute URL")
	}
}

func newTestProxy(t *testing.T) *Proxy {
	t.Helper()
	p, err := New(Options{
		Listen: "127.0.0.1:8888",
		OutDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return p
}
