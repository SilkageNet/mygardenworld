package captureproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
)

const (
	certPagePath = "/"
	caCRTPath    = "/ca.crt"
	caCERPath    = "/ca.cer"
	caDERPath    = "/ca.der"
	caPEMPath    = "/ca.pem"
)

var certPageTemplate = template.Must(template.New("certpage").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>mygardenworld Capture CA</title>
  <style>
    :root {
      color-scheme: light dark;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f6f7f9;
      color: #15171a;
    }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
      box-sizing: border-box;
    }
    main {
      width: min(720px, 100%);
      border: 1px solid #d7dce2;
      border-radius: 8px;
      background: #fff;
      padding: 24px;
      box-shadow: 0 12px 36px rgb(22 29 37 / 10%);
    }
    h1 {
      margin: 0 0 8px;
      font-size: clamp(24px, 5vw, 34px);
      line-height: 1.08;
    }
    p {
      margin: 10px 0;
      line-height: 1.55;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin: 20px 0;
    }
    a.button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-height: 44px;
      padding: 0 16px;
      border-radius: 6px;
      background: #1769e0;
      color: #fff;
      text-decoration: none;
      font-weight: 650;
    }
    a.secondary {
      background: #e9edf3;
      color: #20252b;
    }
    code {
      overflow-wrap: anywhere;
      border-radius: 4px;
      background: #edf1f5;
      padding: 2px 5px;
    }
    dl {
      display: grid;
      grid-template-columns: max-content 1fr;
      gap: 8px 12px;
      margin: 20px 0 0;
    }
    dt {
      color: #596273;
      font-weight: 650;
    }
    dd {
      margin: 0;
      overflow-wrap: anywhere;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        background: #101214;
        color: #f2f4f7;
      }
      main {
        background: #171a1f;
        border-color: #303744;
        box-shadow: none;
      }
      a.secondary {
        background: #29313c;
        color: #f2f4f7;
      }
      code {
        background: #252c36;
      }
      dt {
        color: #b2bac8;
      }
    }
  </style>
</head>
<body>
  <main>
    <h1>mygardenworld Capture CA</h1>
    <p>Download this certificate on your test device, install it as a trusted CA, then set the device HTTP proxy to this same host and port.</p>
    <div class="actions">
      <a class="button" href="{{.CRTPath}}">Download .crt</a>
      <a class="button secondary" href="{{.CERPath}}">Download .cer</a>
      <a class="button secondary" href="{{.PEMPath}}">View PEM</a>
    </div>
    <p>After installing the certificate, keep this process running while you capture game traffic. Stop with <code>Ctrl+C</code>.</p>
    <dl>
      <dt>Proxy</dt>
      <dd><code>{{.Host}}</code></dd>
      <dt>Session</dt>
      <dd><code>{{.SessionDir}}</code></dd>
      <dt>SHA-256</dt>
      <dd><code>{{.Fingerprint}}</code></dd>
    </dl>
  </main>
</body>
</html>`))

func (p *Proxy) serveManagement(w http.ResponseWriter, req *http.Request) bool {
	if req.Method == http.MethodConnect || req.URL == nil {
		return false
	}
	path := req.URL.EscapedPath()
	if path == "" {
		path = certPagePath
	}
	if !isCertificatePagePath(path) {
		return false
	}
	if req.URL.IsAbs() && !p.matchesProxyListenPort(req.URL.Host) {
		return false
	}

	switch path {
	case certPagePath, "/ca", "/ca/":
		p.serveCertPage(w, req)
	case caCRTPath, "/mygardenworld-capture-ca.crt":
		p.serveCACertPEM(w, "mygardenworld-capture-ca.crt")
	case caPEMPath:
		p.serveCAPEM(w)
	case caCERPath:
		p.serveCACertDER(w, "mygardenworld-capture-ca.cer")
	case caDERPath:
		p.serveCACertDER(w, "mygardenworld-capture-ca.der")
	case "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, req)
	}
	return true
}

func isCertificatePagePath(path string) bool {
	switch path {
	case certPagePath, "/ca", "/ca/", caCRTPath, "/mygardenworld-capture-ca.crt", caCERPath, caDERPath, caPEMPath, "/favicon.ico":
		return true
	default:
		return false
	}
}

func (p *Proxy) serveCertPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	data := map[string]string{
		"Host":        req.Host,
		"SessionDir":  p.sessionDir,
		"Fingerprint": caFingerprint(p.ca.Cert.Certificate[0]),
		"CRTPath":     caCRTPath,
		"CERPath":     caCERPath,
		"PEMPath":     caPEMPath,
	}
	_ = certPageTemplate.Execute(w, data)
}

func (p *Proxy) serveCACertPEM(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(p.ca.CertPEM)
}

func (p *Proxy) serveCAPEM(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(p.ca.CertPEM)
}

func (p *Proxy) serveCACertDER(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(p.ca.Cert.Certificate[0])
}

func (p *Proxy) matchesProxyListenPort(hostport string) bool {
	_, targetPort, err := splitHostPortLoose(hostport)
	if err != nil {
		return false
	}
	_, listenPort, err := splitHostPortLoose(p.ListenAddr())
	if err != nil {
		return false
	}
	return targetPort == listenPort
}

func splitHostPortLoose(hostport string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err == nil {
		return strings.Trim(host, "[]"), port, nil
	}
	if strings.Count(hostport, ":") == 1 {
		parts := strings.SplitN(hostport, ":", 2)
		if parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], nil
		}
	}
	return "", "", err
}

func caFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))
	var b strings.Builder
	for i := 0; i < len(hexed); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hexed[i : i+2])
	}
	return b.String()
}
