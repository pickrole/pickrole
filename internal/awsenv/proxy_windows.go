package awsenv

import "github.com/mattn/go-ieproxy"

// systemRoute uses the Windows proxy settings, static proxy or PAC script,
// which is where corporate networks usually configure them.
func systemRoute() Route {
	return Route{Source: SourceWindows, Proxy: ieproxy.GetProxyFunc()}
}
