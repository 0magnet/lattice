package lattice

import (
	"math"
	"strings"
	"testing"
	"time"
)

// A figure is moved the way a solid is turned: what Turned and Posed ask a
// solid about is where forward moved the point to.
func TestFigureTurnsAsTheSolidDoes(t *testing.T) {
	tu := turn{yaw: 0.7, pitch: -0.4, pose: Quat{0.9, 0.1, -0.3, 0.2}}
	l := math.Sqrt(0.81 + 0.01 + 0.09 + 0.04)
	for i := range tu.pose {
		tu.pose[i] /= l
	}
	q := Vec3{0.3, -0.5, 0.2}
	var asked Vec3
	probe := Posed(Turned(func(p Vec3) float64 { asked = p; return 0 }, tu.yaw, tu.pitch), tu.pose)
	probe(tu.forward()(q))
	if d := asked.Add(q.Scale(-1)).Len(); d > 1e-12 {
		t.Fatalf("the solid was asked about %v for the figure's %v", asked, q)
	}
}

// A line along x is a run of ─ across the sheet facing it, a · in a sheet it
// runs through, and nothing in a sheet it misses.
func TestFigureLineGlyphs(t *testing.T) {
	f := new(Figure)
	f.Set([][]Vec3{{{-0.9, 0.01, 0.01}, {0.9, 0.01, 0.01}}})
	const n = 8
	z := renderFigure(Sheet{Axis: Z, Slice: n / 2, N: n}, f, turn{pose: Identity}, Lines, Mono, nil)
	if row := cellsText(z, n/2-1); !strings.Contains(row, "──────────") {
		t.Errorf("z sheet row %d = %q, want a run of ─", n/2-1, row)
	}
	x := renderFigure(Sheet{Axis: X, Slice: n / 2, N: n}, f, turn{pose: Identity}, Lines, Mono, nil)
	if got := strings.Count(cellsText(x, n/2-1), "·"); got != 2 {
		t.Errorf("x sheet: %d ·, want one voxel's two", got)
	}
	y := renderFigure(Sheet{Axis: Y, Slice: 0, N: n}, f, turn{pose: Identity}, Lines, Mono, nil)
	for _, c := range y.Cells {
		if c.Ch != 0 {
			t.Fatalf("y sheet 0 drew %q, which the line never reaches", c.Ch)
		}
	}
}

// A figure of one point is a dot; a changed figure is drawn again.
func TestFigureDotAndRedraw(t *testing.T) {
	p, err := NewProgram([]string{"-n", "4", "-axis", "z", "-slice", "2", "-no-color"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := new(Figure)
	f.Set([][]Vec3{{{0.1, 0.1, 0.1}}})
	p.Show(f)
	var out strings.Builder
	if err := p.Enter(&out); err != nil {
		t.Fatal(err)
	}
	if err := p.Frame(epoch); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "•") {
		t.Fatalf("no dot in %q", out.String())
	}
	out.Reset()
	if err := p.Frame(epoch); err != nil || out.Len() != 0 {
		t.Fatalf("an unchanged figure was drawn again: %q, %v", out.String(), err)
	}
	f.Set([][]Vec3{{{-0.6, 0.1, 0.1}}})
	if err := p.Frame(epoch); err != nil || out.Len() == 0 {
		t.Fatalf("a changed figure was not drawn again: %v", err)
	}
}

func TestReadFigure(t *testing.T) {
	lines, err := ReadFigure(strings.NewReader("# a square\n0 0 0\n10 0 0\n\n5 5 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || len(lines[0]) != 2 || len(lines[1]) != 1 {
		t.Fatalf("got %v", lines)
	}
	if math.Abs(lines[0][1][0]-0.9) > 1e-12 {
		t.Fatalf("not scaled to 0.9: %v", lines[0][1])
	}
	if _, err := ReadFigure(strings.NewReader("1 2\n")); err == nil {
		t.Fatal("two numbers read as a point")
	}
}

// cellsText is row r of f as text.
func cellsText(f Frame, r int) string {
	var b strings.Builder
	for c := range f.Cols {
		if ch := f.At(c, r).Ch; ch != 0 {
			b.WriteRune(ch)
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// epoch is a fixed moment, so a frame drawn twice stands the same way.
var epoch = time.Unix(1_000_000_000, 0)

// BenchmarkFigureVoxels is a trail's worth of short segments, voxelized as a
// stack at 24 rows would.
func BenchmarkFigureVoxels(b *testing.B) {
	pts := make([]Vec3, 20000)
	for i := range pts {
		t := float64(i) / 300
		pts[i] = Vec3{0.8 * math.Sin(t), 0.8 * math.Cos(1.3*t), 0.8 * math.Sin(0.7*t)}
	}
	f := new(Figure)
	for b.Loop() {
		f.Set([][]Vec3{pts})
		f.voxels(24, turn{pose: Identity})
	}
}
