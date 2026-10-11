//go:build !unix

package commands

import "os"

// winch is nothing where there is no SIGWINCH: the size is read once.
func winch() (<-chan os.Signal, func()) { return nil, func() {} }
