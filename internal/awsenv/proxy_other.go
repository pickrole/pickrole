//go:build !linux && !windows

package awsenv

// systemRoute: other systems aren't targets, so only the environment
// variables and the manual setting apply there.
func systemRoute() Route {
	return Route{Source: SourceNone}
}
