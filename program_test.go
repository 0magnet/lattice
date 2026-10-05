package lattice

import (
	"io"
	"strings"
	"testing"
	"time"
)

// The screen sends only what changed: an identical frame sends nothing, and
// one changed cell is one cursor move and one character.
func TestTheScreenSendsOnlyChanges(t *testing.T) {
	var out strings.Builder
	sc := newScreen(&out, false)
	f := Render(Sheet{Axis: Z, Slice: 4, N: 8}, Shapes["sphere"], Lines, Mono)
	if err := sc.draw(f); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("the first frame sent nothing")
	}
	out.Reset()
	g := f
	g.Cells = append([]Cell(nil), f.Cells...)
	if err := sc.draw(g); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("an unchanged frame sent %q", out.String())
	}
	g.Cells = append([]Cell(nil), f.Cells...)
	g.Cells[3*g.Cols+5] = Cell{Ch: 'x'}
	if err := sc.draw(g); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "\x1b[4;6Hx" {
		t.Errorf("one changed cell sent %q", got)
	}
}

// -tile lays the slices out as many to a row as fit, each under its name.
func TestTiled(t *testing.T) {
	p, err := NewProgram([]string{"-n", "4", "-axis", "y", "-tile", "-spin", "0"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	o := p.o
	fs := make([]Frame, 4)
	for k := range fs {
		sh := o.sheet
		sh.Slice = k
		fs[k] = Render(sh, o.solid(time.Unix(0, 0)), o.style, o.coloring)
	}
	f := tiled(fs, Y, 20) // 9 columns a sheet with its gap: two to a row
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
		if _, err := NewProgram(args, io.Discard); err == nil {
			t.Errorf("%v: accepted", args)
		}
	}
}

// A host runs the program frame by frame: Enter takes the screen, Frame at
// one instant twice sends the picture once, and Leave gives it back.
func TestProgramFrameByFrame(t *testing.T) {
	p, err := NewProgram([]string{"-n", "8", "-axis", "x", "-slice", "3"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Frame(time.Unix(0, 0)); err == nil {
		t.Error("Frame before Enter succeeded")
	}
	var out strings.Builder
	if err := p.Enter(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "\x1b[?1049h") {
		t.Errorf("Enter wrote %q", out.String())
	}
	at := time.Unix(100, 0)
	if err := p.Frame(at); err != nil {
		t.Fatal(err)
	}
	n := out.Len()
	if err := p.Frame(at); err != nil {
		t.Fatal(err)
	}
	if out.Len() != n {
		t.Errorf("the same instant twice sent %d more bytes", out.Len()-n)
	}
	if err := p.Leave(); err != nil || !strings.HasSuffix(out.String(), "\x1b[?1049l") {
		t.Errorf("Leave: %v, %q", err, out.String()[out.Len()-12:])
	}
	if s := p.Sheet(); s.Axis != X || s.Slice != 3 || s.N != 8 {
		t.Errorf("sheet %+v", s)
	}
}

// A solid that has not moved is not drawn again: with no spin, the second
// frame renders nothing and sends nothing.
func TestAStillSolidIsNotRedrawn(t *testing.T) {
	p, err := NewProgram([]string{"-n", "8", "-spin", "0"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Enter(&out); err != nil {
		t.Fatal(err)
	}
	if err := p.Frame(time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	n := out.Len()
	if err := p.Frame(time.Unix(500, 0)); err != nil {
		t.Fatal(err)
	}
	if out.Len() != n || !p.drawn {
		t.Errorf("a still solid was drawn again")
	}
}

// Without -n the terminal's size is the resolution: its rows, or half its
// columns if fewer, and the middle slice follows it. -n pins it, and a slice
// given outright is kept within whatever the resolution becomes.
func TestTheTerminalSetsTheResolution(t *testing.T) {
	p, err := NewProgram([]string{"-axis", "y"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	p.Resize(80, 24)
	if s := p.Sheet(); s.N != 24 || s.Slice != 12 {
		t.Errorf("80x24: %+v, want n 24, slice 12", s)
	}
	p.Resize(30, 24) // narrow: half the columns is fewer
	if s := p.Sheet(); s.N != 15 || s.Slice != 7 {
		t.Errorf("30x24: %+v, want n 15, slice 7", s)
	}
	q, err := NewProgram([]string{"-slice", "20"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	q.Resize(200, 50)
	q.Resize(40, 10)
	if s := q.Sheet(); s.N != 10 || s.Slice != 9 {
		t.Errorf("slice 20 at 40x10: %+v, want n 10, slice 9", s)
	}
	r, err := NewProgram([]string{"-n", "8"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	r.Resize(200, 50)
	if r.Sheet().N != 8 {
		t.Errorf("-n 8 was resized to %d", r.Sheet().N)
	}
}

// BenchmarkProgramFrame is one sheet's program drawing a turning solid, as a
// stack at 24 rows runs 72 of.
func BenchmarkProgramFrame(b *testing.B) {
	p, err := NewProgram([]string{"-n", "24", "-axis", "z"}, io.Discard)
	if err != nil {
		b.Fatal(err)
	}
	if err := p.Enter(io.Discard); err != nil {
		b.Fatal(err)
	}
	t := time.Unix(1_000_000_000, 0)
	b.ReportAllocs()
	for b.Loop() {
		t = t.Add(50 * time.Millisecond)
		if err := p.Frame(t); err != nil {
			b.Fatal(err)
		}
	}
}
