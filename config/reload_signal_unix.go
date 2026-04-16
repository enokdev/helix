//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package config

import (
	"os"
	"syscall"
)

func reloadSignals() []os.Signal {
	return []os.Signal{syscall.SIGHUP}
}
