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
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "lattice:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	p, err := lattice.NewProgram(args, os.Stderr)
	if err != nil {
		return err
	}
	if p.Once() {
		return p.Print(os.Stdout, time.Now())
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := p.Enter(os.Stdout); err != nil {
		return err
	}
	defer p.Leave() //nolint:errcheck // the terminal is going away either way
	tick := time.NewTicker(p.Interval())
	defer tick.Stop()
	for {
		if err := p.Frame(time.Now()); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
