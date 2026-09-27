// Package awsenv adapts the AWS clients to the machine's environment: the
// endpoint overrides that the AWS CLI and SDKs honour, so PickRole can be
// pointed at a local fake of AWS (see internal/fakeaws), and the proxy that
// corporate networks require.
package awsenv

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
)

// Service identifiers, as used in AWS_ENDPOINT_URL_<SERVICE>.
const (
	SSO          = "SSO"
	SSOOIDC      = "SSO_OIDC"
	CodeArtifact = "CODEARTIFACT"
)

// Endpoint returns AWS_ENDPOINT_URL_<service>, falling back to
// AWS_ENDPOINT_URL, or nil to use the real AWS endpoint.
func Endpoint(service string) *string {
	for _, name := range []string{"AWS_ENDPOINT_URL_" + service, "AWS_ENDPOINT_URL"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			return &v
		}
	}
	return nil
}

// HTTPClient returns the HTTP client for AWS calls. Its proxy follows the
// latest SetProxy, checked on every request, so a settings change reaches
// clients already created. Loopback hosts never use a proxy.
func HTTPClient() *awshttp.BuildableClient {
	return awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		t.Proxy = func(r *http.Request) (*url.URL, error) {
			return withLoopbackBypass(currentRoute().Proxy)(r)
		}
	})
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
