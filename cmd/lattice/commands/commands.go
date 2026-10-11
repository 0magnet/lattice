// Package commands is the lattice command: it draws one cross-section of a
// turning solid in a terminal.
//
//	lattice --axis z --slice 7          one sheet
//	lattice --axis y --tile             every sheet of the y axis, side by side
//	lattice --shape torus --once        one frame, printed, and done
//
// Run across 3N terminals, one per sheet (--axis x, y and z, each --slice 0 to
// N-1), and stacked in space as transparent sheets, they show the solid in
// depth from every side. Every instance turns the solid by the same clock, so
// they stay in step with nothing between them.
//
// The flags are declared here for cobra, then handed on to lattice.NewProgram
// in the form it parses, so the library keeps one place that knows the
// defaults and the checks.
package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/0magnet/calvin/clihelp"

	"github.com/0magnet/lattice"
)

// RootCmd is the lattice command.
var RootCmd = &cobra.Command{
	Use:           "lattice",
	Short:         "draw one cross-section of a turning solid in a terminal",
	Long:          "Draws one cross-section of a turning solid in a terminal.\n\n  lattice --axis z --slice 7          one sheet\n  lattice --axis y --tile             every sheet of the y axis, side by side\n  lattice --shape torus --once        one frame, printed, and done",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return run(forward(cmd.Flags()))
	},
}

func init() {
	f := RootCmd.Flags()
	f.IntP("n", "n", 0, "voxels along each axis (default the terminal's rows, or half its columns if fewer)")
	f.String("axis", "z", "the sheet's axis: x, y or z")
	f.Int("slice", -1, "the sheet's slice, 0 to n-1 (default the middle)")
	f.String("shape", "sphere", "the solid: "+strings.Join(lattice.ShapeNames(), ", "))
	f.String("figure", "", "draw the lines and points in this file instead of a solid: x y z a line, a blank line between polylines")
	f.String("style", "lines", "how the surface is written: lines, ascii, shade or solid")
	f.Bool("mono", false, "one color, not colored by position")
	f.Bool("no-color", os.Getenv("NO_COLOR") != "", "no color escapes at all")
	f.Float64("spin", 20, "degrees a second the solid turns")
	f.Float64("pitch", 20, "degrees the solid is tilted toward the viewer")
	f.Float64("fps", 20, "frames a second")
	f.Bool("once", false, "print one frame and exit")
	f.Bool("tile", false, "draw every slice of the axis, side by side")
	f.Int("cols", 0, "the terminal's width, for --tile (default $COLUMNS, or 160)")
	clihelp.Init(RootCmd, "lattice")
}

// forward turns the flags that were given back into the command line
// lattice.NewProgram parses. Only the given ones: the rest keep the library's
// own defaults, $NO_COLOR and $COLUMNS included.
func forward(fs *pflag.FlagSet) []string {
	var args []string
	fs.Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "help", "info", "bv":
			return
		}
		args = append(args, "-"+f.Name+"="+f.Value.String())
	})
	return args
}

// Execute runs the command and exits non-zero on error.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lattice:", err)
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
