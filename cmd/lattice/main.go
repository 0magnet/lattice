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
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/0magnet/lattice"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "lattice:", err)
		os.Exit(1)
	}
}

// options are the command line's.
type options struct {
	sheet       lattice.Sheet
	shape       string
	style       lattice.Style
	coloring    lattice.Coloring
	color       bool
	spin, pitch float64 // degrees a second, degrees
	fps         float64
	once, tile  bool
}

func parse(args []string) (options, error) {
	fs := flag.NewFlagSet("lattice", flag.ContinueOnError)
	n := fs.Int("n", 16, "voxels along each axis")
	axis := fs.String("axis", "z", "the sheet's axis: x, y or z")
	slice := fs.Int("slice", -1, "the sheet's slice, 0 to n-1 (default the middle)")
	shape := fs.String("shape", "sphere", "the solid: "+strings.Join(lattice.ShapeNames(), ", "))
	style := fs.String("style", "lines", "how the surface is written: lines, ascii, shade or solid")
	mono := fs.Bool("mono", false, "one color, not colored by position")
	noColor := fs.Bool("no-color", os.Getenv("NO_COLOR") != "", "no color escapes at all")
	spin := fs.Float64("spin", 20, "degrees a second the solid turns")
	pitch := fs.Float64("pitch", 20, "degrees the solid is tilted toward the viewer")
	fps := fs.Float64("fps", 20, "frames a second")
	once := fs.Bool("once", false, "print one frame and exit")
	tile := fs.Bool("tile", false, "draw every slice of the axis, side by side")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	o := options{shape: *shape, color: !*noColor, spin: *spin, pitch: *pitch, fps: *fps, once: *once, tile: *tile}
	if *n < 2 || *n > 256 {
		return o, fmt.Errorf("-n %d: want 2 to 256", *n)
	}
	a, ok := lattice.ParseAxis(*axis)
	if !ok {
		return o, fmt.Errorf("-axis %q: want x, y or z", *axis)
	}
	if *slice < 0 {
		*slice = *n / 2
	}
	if *slice >= *n {
		return o, fmt.Errorf("-slice %d: want 0 to %d", *slice, *n-1)
	}
	o.sheet = lattice.Sheet{Axis: a, Slice: *slice, N: *n}
	if _, ok := lattice.Shapes[*shape]; !ok {
		return o, fmt.Errorf("-shape %q: want one of %s", *shape, strings.Join(lattice.ShapeNames(), ", "))
	}
	if o.style, ok = lattice.Styles[*style]; !ok {
		return o, fmt.Errorf("-style %q: want lines, ascii, shade or solid", *style)
	}
	if *mono {
		o.coloring = lattice.Mono
	}
	if !(o.fps > 0) {
		return o, fmt.Errorf("-fps %v: want more than 0", o.fps)
	}
	return o, nil
}

func run(args []string) error {
	o, err := parse(args)
	if err != nil {
		return err
	}
	if o.once {
		return plain(os.Stdout, o.frame(time.Now()), o.color)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sc := newScreen(os.Stdout, o.color)
	if err := sc.enter(); err != nil {
		return err
	}
	defer sc.leave() //nolint:errcheck // the terminal is going away either way
	tick := time.NewTicker(time.Duration(float64(time.Second) / o.fps))
	defer tick.Stop()
	for {
		if err := sc.draw(o.frame(time.Now())); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

// solid is the shape as it stands at t. The angle comes from the wall clock,
// not from when this process started, so every sheet's process agrees on it.
func (o options) solid(t time.Time) lattice.Shape {
	sec := float64(t.UnixNano()%int64(1e12)) / 1e9 // wraps every ~16 minutes, invisibly at any whole-degree spin
	yaw := math.Mod(o.spin*sec, 360) * math.Pi / 180
	return lattice.Turned(lattice.Shapes[o.shape], yaw, o.pitch*math.Pi/180)
}

// frame is what this process draws at t: its sheet, or with -tile every
// sheet of its axis.
func (o options) frame(t time.Time) lattice.Frame {
	s := o.solid(t)
	if !o.tile {
		return lattice.Render(o.sheet, s, o.style, o.coloring)
	}
	fs := make([]lattice.Frame, o.sheet.N)
	for k := range fs {
		sh := o.sheet
		sh.Slice = k
		fs[k] = lattice.Render(sh, s, o.style, o.coloring)
	}
	return tiled(fs, o.sheet.Axis, termCols())
}

// termCols is the terminal's width as the shell reports it, or 160.
func termCols() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	return 160
}

// tiled lays sheets out left to right and then down, as many to a row as fit
// in cols, each under a label naming it.
func tiled(fs []lattice.Frame, a lattice.Axis, cols int) lattice.Frame {
	if len(fs) == 0 {
		return lattice.Frame{}
	}
	w, h := fs[0].Cols+1, fs[0].Rows+1 // a column between, a label row above
	per := max(1, (cols+1)/w)
	rows := (len(fs) + per - 1) / per
	out := lattice.Frame{Cols: per*w - 1, Rows: rows * h}
	out.Cells = make([]lattice.Cell, out.Cols*out.Rows)
	for k, f := range fs {
		x0, y0 := (k%per)*w, (k/per)*h
		for i, ch := range a.String() + strconv.Itoa(k) {
			if x0+i < out.Cols {
				out.Cells[y0*out.Cols+x0+i] = lattice.Cell{Ch: ch, R: 0x70, G: 0x70, B: 0x70}
			}
		}
		for r := range f.Rows {
			copy(out.Cells[(y0+1+r)*out.Cols+x0:], f.Cells[r*f.Cols:(r+1)*f.Cols])
		}
	}
	return out
}
