//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris

package config

import "os"

func reloadSignals() []os.Signal {
	return nil
}
