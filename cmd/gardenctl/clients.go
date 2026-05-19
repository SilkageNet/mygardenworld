package main

import (
	"net/http"
	"os"

	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
)

// stdHTTPClient returns the HTTP client backing every Connect call. Timeout is
// zero because command-level contexts own deadlines and streams must stay open.
func stdHTTPClient(opts *ctlOpts) *http.Client {
	return &http.Client{Timeout: 0, Transport: authTransport{base: http.DefaultTransport, opts: opts}}
}

func accountClient(opts *ctlOpts) mygardenworldv1connect.AccountServiceClient {
	return mygardenworldv1connect.NewAccountServiceClient(stdHTTPClient(opts), opts.baseURL())
}

func automationClient(opts *ctlOpts) mygardenworldv1connect.AutomationServiceClient {
	return mygardenworldv1connect.NewAutomationServiceClient(stdHTTPClient(opts), opts.baseURL())
}

func policyClient(opts *ctlOpts) mygardenworldv1connect.PolicyServiceClient {
	return mygardenworldv1connect.NewPolicyServiceClient(stdHTTPClient(opts), opts.baseURL())
}

func queryClient(opts *ctlOpts) mygardenworldv1connect.QueryServiceClient {
	return mygardenworldv1connect.NewQueryServiceClient(stdHTTPClient(opts), opts.baseURL())
}

func authClient(opts *ctlOpts) mygardenworldv1connect.AuthServiceClient {
	return mygardenworldv1connect.NewAuthServiceClient(stdHTTPClient(opts), opts.baseURL())
}

type authTransport struct {
	base http.RoundTripper
	opts *ctlOpts
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := t.opts.token()
	if token != "" && req.Header.Get("Authorization") == "" {
		cp := req.Clone(req.Context())
		cp.Header = req.Header.Clone()
		cp.Header.Set("Authorization", "Bearer "+token)
		req = cp
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func (o *ctlOpts) token() string {
	if o.AuthToken != "" {
		return o.AuthToken
	}
	if token := os.Getenv("GARDENCTL_TOKEN"); token != "" {
		return token
	}
	cfg, _ := loadAuthConfig()
	return cfg.AccessToken
}
