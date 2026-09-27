//go:build !windows

package clipboard

// SetSecret is only implemented on Windows.
func SetSecret(string) error { return ErrUnsupported }
