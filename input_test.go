package lattice

import (
	"io"
	"math"
	"testing"
	"time"
)

// A quarter turn about the vertical, as a quaternion, takes the front to the
// right, as Turned's yaw does.
func TestQuatTurnsAsTurnedDoes(t *testing.T) {
	h := math.Sqrt(0.5)
	q := Quat{h, 0, h, 0} // 90° about y
	got := q.rotate(Vec3{0, 0, 1})
	if math.Abs(got[0]-1) > 1e-12 || math.Abs(got[2]) > 1e-12 {
		t.Errorf("front turned to %v, want +x", got)
	}
	s := ball(Vec3{0, 0, 0.6})
	if d := Posed(s, q)(Vec3{0.6, 0, 0}); math.Abs(d+0.2) > 1e-9 {
		t.Errorf("the posed ball is not at +x: %v", d)
	}
}

func newProg(t *testing.T) *Program {
	t.Helper()
	p, err := NewProgram([]string{"-n", "8", "-spin", "0"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A pose arrives as a sequence, possibly in pieces, ended by BEL or by ST.
func TestPoseSequence(t *testing.T) {
	p := newProg(t)
	h := math.Sqrt(0.5)
	seq := PoseSequence(Quat{h, 0, h, 0})
	if p.Input([]byte(seq[:7])); p.pose != Identity {
		t.Fatal("half a sequence was acted on")
	}
	p.Input([]byte(seq[7:]))
	if math.Abs(p.pose[2]-h) > 1e-12 {
		t.Errorf("pose %v", p.pose)
	}
	p.Input([]byte("\x1b]lattice;pose;2;0;0;0\x1b\\"))
	if p.pose != Identity {
		t.Errorf("an ST-ended pose, normalized, is %v", p.pose)
	}
	p.Input([]byte("\x1b]lattice;pose;x;0;0;0\a\x1b]0;a title\a"))
	if p.pose != Identity || len(p.in) != 0 {
		t.Errorf("a bad pose or another program's sequence moved it: %v, %q left", p.pose, p.in)
	}
}

// The arrows turn the solid, r puts it back, and q or ^C ask to quit.
func TestKeys(t *testing.T) {
	p := newProg(t)
	if p.Input([]byte("\x1b[C\x1bOC\x1b[A")) {
		t.Error("arrows asked to quit")
	}
	if math.Abs(p.keyYaw-2*keyStep) > 1e-12 || math.Abs(p.keyPitch+keyStep) > 1e-12 {
		t.Errorf("yaw %v pitch %v", p.keyYaw, p.keyPitch)
	}
	p.Input([]byte("r"))
	if p.keyYaw != 0 || p.keyPitch != 0 {
		t.Error("r did not put it back")
	}
	if !p.Input([]byte("q")) || !p.Input([]byte{3}) {
		t.Error("q or ^C did not ask to quit")
	}
	if p.Input([]byte("\x1b[")); len(p.in) != 2 {
		t.Errorf("an unfinished key was not kept: %q", p.in)
	}
}

// A key or a pose redraws a still solid; nothing else does. A cube, since a
// turned sphere is the same picture and is rightly not sent again.
func TestInputRedraws(t *testing.T) {
	p, err := NewProgram([]string{"-n", "8", "-spin", "0", "-shape", "cube"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var out countWriter
	if err := p.Enter(&out); err != nil {
		t.Fatal(err)
	}
	at := func() int {
		if err := p.Frame(timeZero); err != nil {
			t.Fatal(err)
		}
		return int(out)
	}
	a := at()
	if b := at(); b != a {
		t.Fatal("a still solid was redrawn")
	}
	p.Input([]byte("\x1b[C\x1b[C\x1b[C\x1b[C\x1b[C\x1b[C"))
	if at() == a {
		t.Error("turning it by keys drew nothing")
	}
}

type countWriter int

func (c *countWriter) Write(b []byte) (int, error) { *c += countWriter(len(b)); return len(b), nil }

var timeZero = time.Unix(0, 0)
