package awsenv

import (
	"context"
	"os/exec"
	"time"
)

// systemRoute reads the GNOME proxy settings. Other desktops (KDE, for
// example) aren't read: there, the environment variables or the manual
// setting apply.
func systemRoute() Route {
	cfg, pac, ok := gnomeProxy(gsettings)
	switch {
	case ok:
		return Route{Source: SourceGNOME, Proxy: proxyFunc(cfg)}
	case pac:
		return Route{Source: SourceNone, Note: "proxy.pac_unsupported"}
	default:
		return Route{Source: SourceNone}
	}
}

func gsettings(schema, key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gsettings", "get", schema, key).Output() // #nosec G204 -- fixed program, our own schema and key names
	return string(out), err
}
