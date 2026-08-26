package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func validateServeSecurity(opts serveOpts) error {
	if opts.MaxReqBytes < 0 {
		return errors.New("--max-request-bytes cannot be negative")
	}
	if opts.DebugDir != "" && !opts.InsecureDebug && !isLoopbackListenAddr(opts.ListenAddr) {
		return errors.New("--debug-dir cannot be used with a non-loopback --listen address unless --allow-insecure-debug is set")
	}
	return nil
}

type originPolicy struct {
	allowedOrigins map[string]struct{}
	allowAnyOrigin bool
}

func newOriginPolicy(origins string, allowInsecureAny bool) (originPolicy, error) {
	policy := originPolicy{allowedOrigins: make(map[string]struct{})}
	for origin := range strings.SplitSeq(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			if !allowInsecureAny {
				return originPolicy{}, errors.New("--cors-origins '*' requires --allow-insecure-cors")
			}
			policy.allowAnyOrigin = true
			continue
		}
		normalized, err := canonicalOrigin(origin)
		if err != nil {
			return originPolicy{}, fmt.Errorf("invalid CORS origin %q: %w", origin, err)
		}
		policy.allowedOrigins[normalized] = struct{}{}
	}
	return policy, nil
}

func corsMiddleware(next http.Handler, policy originPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if policy.allows(r, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originGuardMiddleware(next http.Handler, policy originPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !policy.allows(r, origin) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		next.ServeHTTP(w, r)
	})
}

func (p originPolicy) allows(r *http.Request, origin string) bool {
	if p.allowAnyOrigin {
		return true
	}
	normalized, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	if _, ok := p.allowedOrigins[normalized]; ok {
		return true
	}
	return isSameOrigin(r, normalized)
}

func canonicalOrigin(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("origin scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("origin host is required")
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must not include path, query, or fragment")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func isSameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, requestScheme(r)) {
		return false
	}
	originHost, originPort := splitHostPortForCompare(u.Scheme, u.Host)
	requestHost, requestPort := splitHostPortForCompare(requestScheme(r), r.Host)
	return originHost != "" && originHost == requestHost && originPort == requestPort
}

func splitHostPortForCompare(scheme, hostport string) (string, string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
		port = ""
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return "https"
	}
	if forwarded := strings.ToLower(r.Header.Get("Forwarded")); strings.Contains(forwarded, "proto=https") {
		return "https"
	}
	return "http"
}

func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
