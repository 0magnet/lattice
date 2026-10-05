//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// winch is SIGWINCH, how a terminal tells its program its window changed.
func winch() (<-chan os.Signal, func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)
	return c, func() { signal.Stop(c) }
}
