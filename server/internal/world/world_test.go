package world

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

func newWorld(t *testing.T) *World {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	return New(d)
}

const dt = 0.05 // one tick at 20 Hz

var t0 = time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)

// walk steps the simulation until the character stops, checking speed and
// walkability every tick, and returns the areas passed through in order.
func walk(t *testing.T, w *World, id int64, allowRisk bool) (areas []string, seconds float64) {
	t.Helper()
	prev, _ := w.Get(id)
	for ticks := 0; ; ticks++ {
		if ticks > 20*600 {
			t.Fatal("still walking after 10 simulated minutes")
		}
		w.Step(dt)
		seconds += dt
		cur, _ := w.Get(id)
		if moved := dist(Point{prev.X, prev.Z}, Point{cur.X, cur.Z}); moved > w.speed*dt+0.02 {
			t.Fatalf("moved %.3f m in one tick, max %.3f", moved, w.speed*dt)
		}
		a, ok := w.nav.Classify(cur.X, cur.Z)
		if !ok || (a.Zone == ZoneRisk && !allowRisk) {
			t.Fatalf("stood on %v (walkable=%v) at %.2f,%.2f", a, ok, cur.X, cur.Z)
		}
		if len(areas) == 0 || areas[len(areas)-1] != cur.Area {
			areas = append(areas, cur.Area)
		}
		prev = cur
		if !cur.Moving {
			return areas, seconds
		}
	}
}

func TestRouteThroughBlocks(t *testing.T) {
	w := newWorld(t)
	w.Spawn(1, "walker", Point{0, 0}, false, nil)
	route := []struct {
		to   Point
		want []string
	}{
		{Point{-80, 20}, []string{"A", "A-C", "C"}},
		{Point{0, 0}, []string{"C", "A-C", "A"}},
		{Point{80, 80}, []string{"A", "A-E", "E"}},
		{Point{0, 0}, []string{"E", "A-E", "A"}},
		{Point{200, 0}, []string{"A", "A-B", "B"}},
	}
	now := t0
	for _, leg := range route {
		path, err := w.MoveTo(1, leg.to, false, now)
		if err != nil {
			t.Fatalf("move to %v: %v", leg.to, err)
		}
		length := 0.0
		for i := 1; i < len(path); i++ {
			length += dist(path[i-1], path[i])
		}
		areas, secs := walk(t, w, 1, false)
		if !slices.Equal(areas, leg.want) {
			t.Errorf("to %v passed %v, want %v", leg.to, areas, leg.want)
		}
		if got := length / w.speed; secs < got-dt || secs > got+2*dt {
			t.Errorf("to %v took %.2fs for %.1f m, want about %.2fs", leg.to, secs, length, got)
		}
		at, _ := w.Get(1)
		if dist(Point{at.X, at.Z}, leg.to) > 0.01 {
			t.Errorf("stopped at %.2f,%.2f, want %v", at.X, at.Z, leg.to)
		}
		now = now.Add(time.Minute)
	}
}

func TestPathAvoidsObstacles(t *testing.T) {
	w := newWorld(t)
	path, err := w.nav.FindPath(Point{-25, -15}, Point{-5, -15}, false) // A-hall in between
	if err != nil {
		t.Fatal(err)
	}
	if len(path) < 3 {
		t.Fatalf("straight line through a building: %v", path)
	}
	for i := 1; i < len(path); i++ {
		if !w.nav.clear(path[i-1], path[i], false) {
			t.Fatalf("segment %v-%v crosses a blocked area", path[i-1], path[i])
		}
	}
	for name, target := range map[string]Point{
		"inside building": {-15, -15},
		"off map":         {500, 500},
		"open field":      {100, 50},
	} {
		if _, err := w.nav.FindPath(Point{0, 0}, target, true); !errors.Is(err, ErrUnreachable) {
			t.Errorf("%s: err=%v", name, err)
		}
	}
}

func TestRiskAreaNeedsConfirmation(t *testing.T) {
	w := newWorld(t)
	w.Spawn(1, "scout", Point{200, 0}, false, nil)
	inside := Point{200, -130}
	if _, err := w.MoveTo(1, inside, false, t0); !errors.Is(err, ErrRiskRequired) {
		t.Fatalf("entering risk without confirmation: %v", err)
	}
	// The ordinary route to F's safe part never touches the risk area,
	// though the shortcut exists.
	if _, err := w.MoveTo(1, Point{200, -90}, false, t0); err != nil {
		t.Fatal(err)
	}
	walk(t, w, 1, false)
	if _, err := w.MoveTo(1, inside, true, t0.Add(time.Second)); err != nil {
		t.Fatalf("confirmed entry refused: %v", err)
	}
	areas, _ := walk(t, w, 1, true)
	if areas[len(areas)-1] != "F-risk" {
		t.Fatalf("ended in %v", areas)
	}
	// Already inside: leaving needs no new confirmation.
	if _, err := w.MoveTo(1, Point{200, -90}, false, t0.Add(2*time.Second)); err != nil {
		t.Fatalf("leaving the risk area: %v", err)
	}
}

func TestRiskShortcutOnlyWithConfirmation(t *testing.T) {
	w := newWorld(t)
	onShortcut := Point{100, -130}
	if _, err := w.nav.FindPath(Point{0, 0}, onShortcut, false); !errors.Is(err, ErrRiskRequired) {
		t.Fatalf("shortcut road without confirmation: %v", err)
	}
	if _, err := w.nav.FindPath(Point{0, 0}, onShortcut, true); err != nil {
		t.Fatal(err)
	}
}

func pathLength(p []Point) (l float64) {
	for i := 1; i < len(p); i++ {
		l += dist(p[i-1], p[i])
	}
	return l
}

// From A's north edge to F's west corner the risk shortcut is shorter than
// the ordinary road via B; without confirmation the route must still avoid it.
func TestOrdinaryRouteIgnoresShorterRiskShortcut(t *testing.T) {
	w := newWorld(t)
	from, to := Point{0, -25}, Point{172, -140}
	safe, err := w.nav.FindPath(from, to, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(safe); i++ {
		if !w.nav.clear(safe[i-1], safe[i], false) {
			t.Fatalf("unconfirmed route crosses a risk area: %v", safe)
		}
	}
	risky, err := w.nav.FindPath(from, to, true)
	if err != nil {
		t.Fatal(err)
	}
	if pathLength(risky) >= pathLength(safe)-20 {
		t.Fatalf("fixture broken: shortcut %.0f m is not clearly shorter than %.0f m", pathLength(risky), pathLength(safe))
	}
	w.Spawn(1, "courier", from, false, nil)
	if _, err := w.MoveTo(1, to, false, t0); err != nil {
		t.Fatal(err)
	}
	areas, _ := walk(t, w, 1, false)
	if !slices.Contains(areas, "B") || slices.Contains(areas, "F-risk-A") {
		t.Fatalf("ordinary route went %v, want via B and not the shortcut", areas)
	}
}

func TestMoveRequestsAreRateLimitedAndCannotSpeedUp(t *testing.T) {
	w := newWorld(t)
	w.Spawn(1, "spammer", Point{0, 0}, false, nil)
	targets := []Point{{25, 0}, {-25, 0}}
	accepted := 0
	for i := range 100 {
		now := t0.Add(time.Duration(i) * 10 * time.Millisecond) // 100 requests in 1 s
		if _, err := w.MoveTo(1, targets[i%2], false, now); err == nil {
			accepted++
		} else if !errors.Is(err, ErrRateLimited) {
			t.Fatal(err)
		}
		if i%5 == 4 {
			w.Step(dt)
		}
	}
	if accepted != w.limit {
		t.Fatalf("accepted %d requests in one second, limit %d", accepted, w.limit)
	}
	at, _ := w.Get(1)
	if d := dist(Point{0, 0}, Point{at.X, at.Z}); d > w.speed*1.0+0.01 {
		t.Fatalf("moved %.2f m in one simulated second", d)
	}
	if _, err := w.MoveTo(1, targets[0], false, t0.Add(3*time.Second)); err != nil {
		t.Fatalf("limit must reset: %v", err)
	}
}

func TestSpawnDetachAndSnapshots(t *testing.T) {
	w := newWorld(t)
	at, a := w.Spawn(1, "lost", Point{100, 50}, false, nil) // no longer walkable
	if at != w.spawn || a.ID != "A" {
		t.Fatalf("unwalkable save must respawn at A: %v %v", at, a)
	}
	w.Spawn(2, "near", Point{20, 0}, false, &testSink{})
	w.Spawn(3, "far", Point{200, 0}, false, nil)
	_, _ = w.MoveTo(1, Point{-80, 0}, false, t0)
	w.Detach(1, &testSink{}) // a different sink: ignored
	if v, _ := w.Get(1); !v.Moving {
		t.Fatal("a stale connection must not stop the character")
	}
	w.Detach(1, nil)
	if v, _ := w.Get(1); v.Moving {
		t.Fatal("a detached character must stop")
	}
	if _, err := w.MoveTo(9, Point{0, 0}, false, t0); !errors.Is(err, ErrNotInWorld) {
		t.Fatal("unknown character moved")
	}
	snaps := w.snapshots()
	if len(snaps) != 1 {
		t.Fatalf("only connected characters get snapshots, got %d", len(snaps))
	}
	var ids []int64
	for _, e := range snaps[0].snap.Entities {
		ids = append(ids, e.ID)
	}
	if !slices.Equal(ids, []int64{1, 2}) {
		t.Fatalf("view radius: saw %v, want [1 2]", ids)
	}
	if p, dead, ok := w.Despawn(2); !ok || dead || p != (Point{20, 0}) {
		t.Fatalf("despawn returned %v %v", p, ok)
	}
}

// testSink is a pointer type, like the real connection: sinks are compared.
type testSink struct{ pushes int }

func (s *testSink) Push(string, any) { s.pushes++ }
