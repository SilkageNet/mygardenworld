package notification

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrUnsafeEndpoint = errors.New("通知地址仅支持公网 HTTPS，不允许内网地址、凭据字段或重定向")

func ValidateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 4096 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return ErrUnsafeEndpoint
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return ErrUnsafeEndpoint
		}
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicAddress(ip) {
		return ErrUnsafeEndpoint
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") && !strings.Contains(host, ":") {
		return ErrUnsafeEndpoint
	}
	return nil
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("168.63.129.16/32"), // Azure host services virtual address.
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Zone() != "" {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

type lookupIP func(context.Context, string) ([]net.IPAddr, error)

func dialPublic(ctx context.Context, network, address string, lookup lookupIP) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrUnsafeEndpoint
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, ErrUnsafeEndpoint
	}
	// Reject mixed answers too, then dial an already validated numeric address.
	// The TLS transport still verifies the original hostname. No second DNS
	// resolution, environmental HTTP proxy or redirect can bypass this gate.
	for _, candidate := range ips {
		ip, ok := netip.AddrFromSlice(candidate.IP)
		if !ok || candidate.Zone != "" || !publicAddress(ip) {
			return nil, ErrUnsafeEndpoint
		}
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	for _, candidate := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	return nil, err
}

func safeClient() *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialPublic(ctx, network, address, net.DefaultResolver.LookupIPAddr)
		},
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 16 << 10,
	}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
