package captureproxy

import (
	"path"
	"strings"
)

type hostFilter struct {
	patterns []string
}

func newHostFilter(patterns []string) hostFilter {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return hostFilter{patterns: out}
}

func (f hostFilter) Match(hostport string) bool {
	if len(f.patterns) == 0 {
		return true
	}
	host := strings.ToLower(stripPort(hostport))
	if host == "" {
		return false
	}
	for _, p := range f.patterns {
		switch {
		case p == "*":
			return true
		case strings.HasPrefix(p, "*."):
			suffix := strings.TrimPrefix(p, "*")
			if strings.HasSuffix(host, suffix) && host != strings.TrimPrefix(suffix, ".") {
				return true
			}
		case strings.HasPrefix(p, "."):
			if strings.HasSuffix(host, p) || host == strings.TrimPrefix(p, ".") {
				return true
			}
		case strings.Contains(p, "*"):
			if ok, _ := path.Match(p, host); ok {
				return true
			}
		case host == p:
			return true
		}
	}
	return false
}
