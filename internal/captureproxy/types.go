package captureproxy

import (
	"net"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultListen          = "127.0.0.1:8888"
	DefaultOutDir          = "captures"
	DefaultHostPatterns    = "*.babigame.cn,babigame.cn"
	DefaultMaxBodyBytes    = int64(1024 * 1024)
	DefaultMaxWSFrameBytes = int64(1024 * 1024)
)

// Options controls one capture proxy session.
type Options struct {
	Listen          string
	OutDir          string
	SessionName     string
	CACertPath      string
	CAKeyPath       string
	HostPatterns    []string
	MaxBodyBytes    int64
	MaxWSFrameBytes int64
	Verbose         bool
}

// Normalize fills defaults and canonicalises list-like fields.
func (o Options) Normalize() Options {
	if strings.TrimSpace(o.Listen) == "" {
		o.Listen = DefaultListen
	}
	if strings.TrimSpace(o.OutDir) == "" {
		o.OutDir = DefaultOutDir
	}
	if len(o.HostPatterns) == 0 {
		o.HostPatterns = splitPatterns(DefaultHostPatterns)
	}
	if o.MaxBodyBytes < 0 {
		o.MaxBodyBytes = 0
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if o.MaxWSFrameBytes < 0 {
		o.MaxWSFrameBytes = 0
	}
	if o.MaxWSFrameBytes == 0 {
		o.MaxWSFrameBytes = DefaultMaxWSFrameBytes
	}
	return o
}

func splitPatterns(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func sessionSlug(name string, now time.Time) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "capture"
	}
	repl := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-", " ", "-")
	name = repl.Replace(name)
	name = strings.Trim(name, ".-")
	if name == "" {
		name = "capture"
	}
	return now.Format("20060102-150405") + "_" + name
}

func stripPort(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(host, "[]")
	}
	if i := strings.IndexByte(hostport, ':'); i > -1 && strings.Count(hostport, ":") == 1 {
		return hostport[:i]
	}
	return strings.Trim(hostport, "[]")
}

func joinPath(parts ...string) string {
	return filepath.Clean(filepath.Join(parts...))
}
