package main

import (
	"strings"
	"testing"
	"time"

	"github.com/0magnet/lattice"
)

// The screen sends only what changed: an identical frame sends nothing, and
// one changed cell is one cursor move and one character.
func TestTheScreenSendsOnlyChanges(t *testing.T) {
	var out strings.Builder
	sc := newScreen(&out, false)
	f := lattice.Render(lattice.Sheet{Axis: lattice.Z, Slice: 4, N: 8}, lattice.Shapes["sphere"], lattice.Lines, lattice.Mono)
	if err := sc.draw(f); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("the first frame sent nothing")
	}
	out.Reset()
	g := f
	g.Cells = append([]lattice.Cell(nil), f.Cells...)
	if err := sc.draw(g); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("an unchanged frame sent %q", out.String())
	}
	g.Cells = append([]lattice.Cell(nil), f.Cells...)
	g.Cells[3*g.Cols+5] = lattice.Cell{Ch: 'x'}
	if err := sc.draw(g); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "\x1b[4;6Hx" {
		t.Errorf("one changed cell sent %q", got)
	}
}

// -tile lays the slices out as many to a row as fit, each under its name.
func TestTiled(t *testing.T) {
	o, err := parse([]string{"-n", "4", "-axis", "y", "-tile", "-spin", "0"})
	if err != nil {
		t.Fatal(err)
	}
	fs := make([]lattice.Frame, 4)
	for k := range fs {
		sh := o.sheet
		sh.Slice = k
		fs[k] = lattice.Render(sh, o.solid(time.Unix(0, 0)), o.style, o.coloring)
	}
	f := tiled(fs, lattice.Y, 20) // 9 columns a sheet with its gap: two to a row
	if f.Cols != 17 || f.Rows != 10 {
		t.Fatalf("%dx%d, want 17x10", f.Cols, f.Rows)
	}
	if f.At(0, 0).Ch != 'y' || f.At(1, 0).Ch != '0' || f.At(9, 5).Ch != 'y' || f.At(10, 5).Ch != '3' {
		t.Error("the labels are not where the sheets are")
	}
}

func TestParseRejects(t *testing.T) {
	for _, args := range [][]string{
		{"-axis", "w"}, {"-n", "1"}, {"-n", "8", "-slice", "8"},
		{"-shape", "teapot"}, {"-style", "fancy"}, {"-fps", "0"},
	} {
		if _, err := parse(args); err == nil {
			t.Errorf("%v: accepted", args)
		}
	}
}
