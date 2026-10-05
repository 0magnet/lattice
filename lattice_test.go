package lattice

import (
	"math"
	"testing"
)

// Every family's basis is orthonormal and right-handed, so each is read
// unmirrored from its own side.
func TestBasesAreRightHanded(t *testing.T) {
	for _, a := range []Axis{X, Y, Z} {
		b := BasisOf(a)
		for _, v := range []Vec3{b.Right, b.Up, b.Normal} {
			if math.Abs(v.Len()-1) > 1e-12 {
				t.Errorf("%v: %v is not a unit vector", a, v)
			}
		}
		if b.Right.Dot(b.Up) != 0 || b.Right.Cross(b.Up) != b.Normal {
			t.Errorf("%v: right × up = %v, normal %v", a, b.Right.Cross(b.Up), b.Normal)
		}
	}
}

// The three families cut the same voxels: across all 3N sheets every voxel
// center turns up exactly three times, once in a sheet of each axis.
func TestTheThreeFamiliesShareTheirVoxels(t *testing.T) {
	const n = 6
	seen := map[[3]int]map[Axis]int{}
	for _, a := range []Axis{X, Y, Z} {
		for k := range n {
			s := Sheet{Axis: a, Slice: k, N: n}
			for v := range n {
				for u := range n {
					p := s.Voxel(u, v)
					var key [3]int
					for i := range 3 {
						key[i] = int(math.Round((p[i] + 1) / 2 * n * 2)) // odd: a voxel center
					}
					if seen[key] == nil {
						seen[key] = map[Axis]int{}
					}
					seen[key][a]++
				}
			}
		}
	}
	if len(seen) != n*n*n {
		t.Fatalf("%d distinct voxel centers, want %d", len(seen), n*n*n)
	}
	for k, m := range seen {
		if m[X] != 1 || m[Y] != 1 || m[Z] != 1 {
			t.Errorf("voxel %v: in %v", k, m)
		}
	}
}

// ball is a small ball at c.
func ball(c Vec3) Shape {
	return func(p Vec3) float64 { return p.Add(c.Scale(-1)).Len() - 0.2 }
}

// litColumns and litRows are where a frame has anything, as the mean column
// and row of its lit cells.
func centroid(f Frame) (c, r float64) {
	n := 0
	for i, cell := range f.Cells {
		if cell.Ch != 0 {
			c += float64(i % f.Cols)
			r += float64(i / f.Cols)
			n++
		}
	}
	if n == 0 {
		return -1, -1
	}
	return c / float64(n), r / float64(n)
}

// A ball in front of the middle is to the left on a side sheet (seen from
// +x) and at the bottom on a level sheet (seen from above).
func TestEachFamilyIsSeenFromItsOwnSide(t *testing.T) {
	const n = 16
	s := ball(Vec3{0, 0, 0.6})
	c, _ := centroid(Render(Sheet{Axis: X, Slice: n / 2, N: n}, s, Solid, Mono))
	if c > float64(n) {
		t.Errorf("x sheet: the front is at column %v of %d, want the left half", c, 2*n)
	}
	_, r := centroid(Render(Sheet{Axis: Y, Slice: n / 2, N: n}, s, Solid, Mono))
	if r < float64(n)/2 {
		t.Errorf("y sheet: the front is at row %v of %d, want the bottom half", r, n)
	}
	c, r = centroid(Render(Sheet{Axis: Z, Slice: n - 2, N: n}, ball(Vec3{0.6, 0.6, 0.8}), Solid, Mono))
	if c < float64(n) || r > float64(n)/2 {
		t.Errorf("z sheet: up and right is at column %v, row %v", c, r)
	}
}

// The sphere's middle sheet is a ring of the right size, written as the line
// the surface makes: flat at the top, upright at the side.
func TestTheSphereIsARing(t *testing.T) {
	const n = 16
	sh := Sheet{Axis: Z, Slice: n / 2, N: n}
	f := Render(sh, Shapes["sphere"], Lines, Mono)
	for v := range n {
		for u := range n {
			lit := f.At(2*u, v).Ch != 0
			r := math.Hypot(sh.Voxel(u, v)[0], sh.Voxel(u, v)[1])
			if lit && math.Abs(r-0.8) > 2.0/n {
				t.Errorf("voxel %d,%d lit at radius %.2f", u, v, r)
			}
		}
	}
	if ch := f.At(n, 1).Ch; ch != '─' {
		t.Errorf("top of the ring is %q, want ─", ch)
	}
	if ch := f.At(2, n/2).Ch; ch != '│' {
		t.Errorf("side of the ring is %q, want │", ch)
	}
	far := Render(Sheet{Axis: Z, Slice: 0, N: n}, Shapes["sphere"], Lines, Mono)
	if c, _ := centroid(far); c != -1 {
		t.Errorf("a sheet beyond the sphere has something on it")
	}
}

// A voxel is two equal characters, so it is square in a terminal, and the
// same voxel is the same color in every sheet through it.
func TestAVoxelIsTwoCharactersAndOneColor(t *testing.T) {
	const n = 8
	s := Shapes["sphere"]
	f := Render(Sheet{Axis: Z, Slice: 1, N: n}, s, Lines, ByPosition)
	if f.Cols != 2*n || f.Rows != n {
		t.Fatalf("%dx%d", f.Cols, f.Rows)
	}
	for i := 0; i < len(f.Cells); i += 2 {
		if f.Cells[i] != f.Cells[i+1] {
			t.Errorf("cell %d: %v and %v", i, f.Cells[i], f.Cells[i+1])
		}
	}
	p := Sheet{Axis: Z, Slice: 1, N: n}.Voxel(3, 2)
	r1, g1, b1 := color(p, ByPosition)
	q := Sheet{Axis: X, Slice: 3, N: n}.Voxel(n-2, 2) // the same voxel, from the side
	if p != q {
		t.Fatalf("not the same voxel: %v %v", p, q)
	}
	if r2, g2, b2 := color(q, ByPosition); r1 != r2 || g1 != g2 || b1 != b2 {
		t.Error("one voxel, two colors")
	}
}

// Turning by a whole turn changes nothing, and a quarter turn about the
// vertical moves what was in front to the side.
func TestTurned(t *testing.T) {
	s := ball(Vec3{0, 0, 0.6})
	if d := Turned(s, 2*math.Pi, 0)(Vec3{0, 0, 0.6}); math.Abs(d+0.2) > 1e-9 {
		t.Errorf("a whole turn moved it: %v", d)
	}
	q := Turned(s, math.Pi/2, 0)
	if q(Vec3{0.6, 0, 0}) > 0 && q(Vec3{-0.6, 0, 0}) > 0 {
		t.Error("a quarter turn did not bring it to either side")
	}
}

// Render's corner slab finds the same voxels Lit does, and the three
// families, each rendered on its own, light the same voxels: exactly, for
// every shape, turned to an awkward angle.
func TestTheFamiliesLightTheSameVoxels(t *testing.T) {
	const n = 10
	for _, name := range ShapeNames() {
		s := Turned(Shapes[name], 0.7, 0.3)
		lit := map[Axis]map[[3]int]bool{}
		for _, a := range []Axis{X, Y, Z} {
			lit[a] = map[[3]int]bool{}
			for k := range n {
				sh := Sheet{Axis: a, Slice: k, N: n}
				f := Render(sh, s, Lines, Mono)
				for v := range n {
					for u := range n {
						on := f.At(2*u, v).Ch != 0
						if on != Lit(s, sh.Voxel(u, v), n) {
							t.Fatalf("%s %v%d voxel %d,%d: slab %v, Lit %v", name, a, k, u, v, on, !on)
						}
						if on {
							lit[a][sh.index(u, v)] = true
						}
					}
				}
			}
		}
		if len(lit[Z]) == 0 {
			t.Errorf("%s: nothing lit", name)
		}
		for _, a := range []Axis{X, Y} {
			if len(lit[a]) != len(lit[Z]) {
				t.Errorf("%s: %v lights %d voxels, z %d", name, a, len(lit[a]), len(lit[Z]))
			}
			for i := range lit[Z] {
				if !lit[a][i] {
					t.Errorf("%s: voxel %v lit in z, not in %v", name, i, a)
				}
			}
		}
	}
}

func BenchmarkRender(b *testing.B) {
	sh := Sheet{Axis: Z, Slice: 12, N: 24}
	s := Turned(Shapes["torus"], 0.5, 0.3)
	for b.Loop() {
		Render(sh, s, Lines, ByPosition)
	}
}
