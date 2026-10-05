// Command lattice draws one cross-section of a turning solid in a terminal.
//
//	lattice -axis z -slice 7          one sheet
//	lattice -axis y -tile             every sheet of the y axis, side by side
//	lattice -shape torus -once        one frame, printed, and done
//
// Run across 3N terminals, one per sheet (-axis x, y and z, each -slice 0 to
// N-1), and stacked in space as transparent sheets, they show the solid in
// depth from every side. Every instance turns the solid by the same clock, so
// they stay in step with nothing between them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0magnet/lattice"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "lattice:", err)
		}
		os.Exit(1)
	}
}

// readKeys reads the terminal a key at a time — raw, so a key arrives when it
// is pressed and not when Enter is — and hands what it reads to the frame
// loop, which alone touches the program. Nothing, when stdin is not a
// terminal. restore puts the terminal back as it was, and has to be called
// before the program exits, not left to a goroutine that may not get to run.
func readKeys(ctx context.Context) (keys <-chan []byte, restore func()) {
	ch := make(chan []byte)
	fd := int(os.Stdin.Fd()) //nolint:gosec // a file descriptor fits an int
	if !term.IsTerminal(fd) {
		return ch, func() {}
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return ch, func() {}
	}
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			select {
			case ch <- append([]byte(nil), buf[:n]...):
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, func() { _ = term.Restore(fd, old) } //nolint:errcheck // the terminal is going away either way
}

func run(args []string) error {
	p, err := lattice.NewProgram(args, os.Stderr)
	if err != nil {
		return err
	}
	// The terminal's size is the resolution, unless -n says otherwise.
	size := func() {
		if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil { //nolint:gosec // a file descriptor fits an int
			p.Resize(w, h)
		}
	}
	size()
	if p.Once() {
		return p.Print(os.Stdout, time.Now())
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := p.Enter(os.Stdout); err != nil {
		return err
	}
	defer p.Leave() //nolint:errcheck // the terminal is going away either way
	keys, restore := readKeys(ctx)
	defer restore()
	resized, unwatch := winch()
	defer unwatch()
	tick := time.NewTicker(p.Interval())
	defer tick.Stop()
	for {
		if err := p.Frame(time.Now()); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-resized:
			size()
		case b := <-keys:
			if p.Input(b) {
				return nil
			}
		case <-tick.C:
		}
	}
}
