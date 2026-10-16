package world

import (
	"testing"
	"time"
)

// leaveWorld returns a world with short departure delays and a recorder of
// characters that have left.
func leaveWorld(t *testing.T) (*World, *[]int64) {
	t.Helper()
	w := newWorld(t)
	w.SafeDisconnect, w.Linger = time.Second, 2*time.Second
	var left []int64
	w.Hooks.OnLeft = func(id int64) { left = append(left, id); w.Despawn(id) }
	return w, &left
}

func steps(w *World, seconds float64) {
	for range int(seconds / dt) {
		w.Step(dt)
	}
}

func TestLogoutInSafeZoneIsImmediate(t *testing.T) {
	w, _ := leaveWorld(t)
	w.Spawn(1, "安者", Point{0, 0}, false, &testSink{})
	w.Step(dt)
	if now, err := w.RequestLeave(1); err != nil || !now {
		t.Fatalf("safe-zone logout out of combat must be immediate: %v %v", now, err)
	}
}

func TestLogoutInTheWildWaitsAndMovingCancels(t *testing.T) {
	w, left := leaveWorld(t)
	sink := &recordSink{}
	w.Spawn(1, "行人", Point{100, 0}, false, sink)
	w.Step(dt)
	if now, _ := w.RequestLeave(1); now {
		t.Fatal("a logout on the road must be delayed")
	}
	steps(w, 1)
	if _, err := w.MoveTo(1, Point{110, 0}, false, t0); err != nil {
		t.Fatal(err)
	}
	if !sink.got("leave_cancelled") {
		t.Fatal("moving during a logout must cancel it and tell the player")
	}
	steps(w, 5)
	if len(*left) != 0 {
		t.Fatal("cancelled logout still left")
	}
	w.Stop(1)
	w.RequestLeave(1)
	steps(w, 1.9)
	if len(*left) != 0 {
		t.Fatal("left before the 2 s linger")
	}
	steps(w, 0.2)
	if len(*left) != 1 || !sink.got("left_world") {
		t.Fatalf("should have left after the linger: %v", *left)
	}
}

func TestCombatDelaysAndAttackCancelsLogout(t *testing.T) {
	w, left := leaveWorld(t)
	w.Spawn(1, "甲", Point{200, -130}, false, &testSink{}) // risk area: PvP allowed
	w.Spawn(2, "乙", Point{201.2, -130}, false, &recordSink{})
	w.Step(dt)
	if err := w.Cast(1, "basic", Point{}, 2); err != nil {
		t.Fatal(err)
	}
	steps(w, 1)
	if now, _ := w.RequestLeave(2); now {
		t.Fatal("logout in combat must be delayed")
	}
	// Disengaging (PvP 30 s) comes first, then the 2 s linger.
	steps(w, 30)
	if len(*left) != 0 {
		t.Fatal("left while still in combat")
	}
	steps(w, 3)
	if len(*left) != 1 || (*left)[0] != 2 {
		t.Fatalf("should leave after disengage + linger: %v", *left)
	}

	// An attack after the request cancels a logout outright.
	w2, left2 := leaveWorld(t)
	w2.Spawn(1, "甲", Point{200, -130}, false, &testSink{})
	victim := &recordSink{}
	w2.Spawn(2, "乙", Point{201.2, -130}, false, victim)
	w2.Step(dt)
	w2.RequestLeave(2)
	if err := w2.Cast(1, "basic", Point{}, 2); err != nil {
		t.Fatal(err)
	}
	steps(w2, 40)
	if len(*left2) != 0 || !victim.got("leave_cancelled") {
		t.Fatalf("being attacked must cancel the logout: left=%v", *left2)
	}
}

func TestDisconnectLingerRestartsWhenAttacked(t *testing.T) {
	w, left := leaveWorld(t)
	w.Spawn(1, "甲", Point{200, -130}, false, &testSink{})
	away := &testSink{}
	w.Spawn(2, "掉线者", Point{201.2, -130}, false, away)
	w.Step(dt)
	w.Detach(2, away) // network loss in a risk area: no auto-combat, linger 2 s
	steps(w, 1)
	if err := w.Cast(1, "basic", Point{}, 2); err != nil {
		t.Fatal(err)
	}
	steps(w, 25)
	if len(*left) != 0 {
		t.Fatal("an attacked disconnected character must stay until it disengages")
	}
	if v, _ := w.Get(2); v.Action != "" || v.Moving {
		t.Fatalf("a disconnected character must not act on its own: %+v", v)
	}
	steps(w, 8) // 30 s PvP disengage from the hit, then 2 s
	if len(*left) != 1 {
		t.Fatalf("should leave after disengage + linger: %v", *left)
	}
}

func TestSafeDisconnectAndReconnectCancels(t *testing.T) {
	w, left := leaveWorld(t)
	s := &testSink{}
	w.Spawn(1, "镇民", Point{0, 0}, false, s)
	w.Step(dt)
	w.Detach(1, s)
	steps(w, 0.5)
	w.Attach(1, &testSink{}) // came back in time
	steps(w, 3)
	if len(*left) != 0 {
		t.Fatal("reconnecting must cancel the departure")
	}
	w.Detach(1, nil) // a different (stale) sink: ignored
	steps(w, 3)
	if len(*left) != 0 {
		t.Fatal("a stale connection's disconnect must be ignored")
	}
}

type recordSink struct{ types []string }

func (r *recordSink) Push(typ string, _ any) { r.types = append(r.types, typ) }
func (r *recordSink) got(typ string) bool {
	for _, t := range r.types {
		if t == typ {
			return true
		}
	}
	return false
}

// Combat carried into town: a logout waits for the disengage, then leaves at
// once (a safe-zone logout has no extra linger).
func TestLogoutInTownAfterFightLeavesOnDisengage(t *testing.T) {
	w, left := leaveWorld(t)
	w.SafeDisconnect = 5 * time.Second // make any wrong extra wait obvious
	w.Spawn(1, "逃兵", Point{60, 0}, false, &testSink{})
	m := w.eng.NewMonster(9, w.eng.MonsterKind("bandit"), vec(Point{61.5, 0}))
	w.eng.Add(m)
	w.ents[9] = &entity{id: 9, name: "山贼", pos: Point{61.5, 0}, f: m}
	w.Step(dt)
	if err := w.eng.Cast(9, "basic", vec(Point{}), 1); err != nil {
		t.Fatal(err)
	}
	steps(w, 1)
	// Run into town (teleported here; walking would take a while).
	w.ents[1].pos, w.ents[1].f.Pos = Point{0, 0}, vec(Point{0, 0})
	w.Step(dt)
	if now, _ := w.RequestLeave(1); now {
		t.Fatal("still in combat: the logout must wait")
	}
	// The hit landed 0.4 s after the cast; PvE disengage is 15 s after the
	// hit, i.e. 15.4 s after the cast. 1.05 s have passed already.
	steps(w, 14)
	if len(*left) != 0 {
		t.Fatal("left before disengaging")
	}
	steps(w, 0.5)
	if len(*left) != 1 {
		t.Fatalf("should leave right after disengaging in town, got %v", *left)
	}
}
