package world

import (
	"errors"
	"testing"
	"time"

	"github.com/leiouliao-coder/jianghu/server/internal/combat"
)

// duel puts two characters 1.2 m apart at x and returns the world.
func duelAt(t *testing.T, x, z float64) *World {
	t.Helper()
	w := newWorld(t)
	w.Spawn(1, "甲", Point{x, z}, false, nil)
	w.Spawn(2, "乙", Point{x + 1.2, z}, false, nil)
	w.Step(dt) // classify areas
	return w
}

func hitOnce(t *testing.T, w *World) int {
	t.Helper()
	before, _ := w.Get(2)
	if err := w.Cast(1, "basic", Point{}, 2); err != nil {
		return 0
	}
	for range 20 {
		w.Step(dt)
	}
	after, _ := w.Get(2)
	return before.HP - after.HP
}

func TestZoneRulesDecideWhoCanBeHarmed(t *testing.T) {
	cases := []struct {
		name string
		at   Point
		harm bool
	}{
		{"service town A", Point{0, 0}, false},
		{"pve road", Point{100, 0}, false},    // players never fight outside risk areas
		{"risk area", Point{200, -130}, true}, // both inside F-risk
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := duelAt(t, tc.at[0], tc.at[1])
			if got := hitOnce(t, w) > 0; got != tc.harm {
				t.Fatalf("harm=%v want %v", got, tc.harm)
			}
		})
	}
}

func TestMonsterRulesAndServiceZone(t *testing.T) {
	w := newWorld(t)
	p := w.eng.NewPlayer(1, combat.Vec{X: 100}, combat.Vec{X: 1}, "armed")
	m := w.eng.NewMonster(9, w.eng.MonsterKind("bandit"), combat.Vec{X: 101})
	w.ents[1] = &entity{id: 1, f: p, area: Area{ID: "A-B", Zone: ZonePvE}}
	w.ents[9] = &entity{id: 9, f: m, area: Area{ID: "A-B", Zone: ZonePvE}}
	if !w.CanHarm(p, m) || !w.CanHarm(m, p) {
		t.Fatal("players and monsters fight outside service zones")
	}
	w.ents[1].area = Area{ID: "A", Zone: ZoneService}
	if w.CanHarm(m, p) || w.CanHarm(p, m) {
		t.Fatal("nobody harms or is harmed inside a service zone")
	}
}

func TestDeathRespawnAndCorpse(t *testing.T) {
	w := duelAt(t, 200, -130)
	clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	w.Now = func() time.Time { return clock }
	var deaths []int64
	var expired []int64
	w.Hooks.OnDeath = func(id int64, name string, at Point) {
		deaths = append(deaths, id)
		w.AddCorpse(500, at, name, clock.Add(CorpseTTL), false)
	}
	w.Hooks.OnCorpseExpired = func(id int64) { expired = append(expired, id) }
	w.ents[2].f.HP = 50
	hitOnce(t, w)
	for range 40 {
		w.Step(dt)
	}
	if len(deaths) != 1 || deaths[0] != 2 {
		t.Fatalf("death hook ran %v, want once for 2", deaths)
	}
	v, _ := w.Get(2)
	if !v.Dead || v.HP != 0 {
		t.Fatalf("victim view %+v", v)
	}
	if _, err := w.MoveTo(2, Point{200, -120}, true, t0); !errors.Is(err, ErrBusy) {
		t.Fatalf("the dead cannot walk: %v", err)
	}
	if err := w.Cast(2, "basic", Point{}, 1); err == nil {
		t.Fatal("the dead cannot fight")
	}
	if err := w.CanLoot(2, 500); !errors.Is(err, ErrDead) {
		t.Fatalf("the dead cannot loot: %v", err)
	}
	if err := w.CanLoot(1, 500); err != nil {
		t.Fatalf("killer standing next to the corpse may loot: %v", err)
	}
	w.Spawn(3, "远人", Point{200, -90}, false, nil)
	w.Step(dt)
	if err := w.CanLoot(3, 500); !errors.Is(err, ErrTooFar) {
		t.Fatalf("looting from 40 m: %v", err)
	}

	at, area, err := w.Respawn(2)
	if err != nil || area.Zone != ZoneService {
		t.Fatalf("respawn at %v %v %v", at, area, err)
	}
	if at != (Point{200, 0}) {
		t.Fatalf("respawn at nearest safe point B (200,0), got %v", at)
	}
	if v, _ := w.Get(2); v.Dead || v.HP != v.MaxHP {
		t.Fatalf("respawned with full health: %+v", v)
	}
	if _, _, err := w.Respawn(2); !errors.Is(err, ErrNotDead) {
		t.Fatal("respawning the living")
	}

	// The corpse's 30 minutes count from death; nothing extends them.
	clock = clock.Add(CorpseTTL - time.Second)
	w.Step(dt)
	if len(expired) != 0 {
		t.Fatal("expired early")
	}
	clock = clock.Add(time.Second)
	w.Step(dt)
	if len(expired) != 1 || expired[0] != 500 {
		t.Fatalf("expiry hook %v", expired)
	}
	if err := w.CanLoot(1, 500); !errors.Is(err, ErrTooFar) {
		t.Fatal("expired corpse still lootable")
	}
}

func TestEmptiedCorpseFadesAfterSixtySeconds(t *testing.T) {
	w := newWorld(t)
	clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	w.Now = func() time.Time { return clock }
	w.Spawn(1, "拾者", Point{0, 0}, false, nil)
	w.AddCorpse(7, Point{1, 0}, "旧主", clock.Add(CorpseTTL), false)
	w.CorpseEmptied(7)
	clock = clock.Add(59 * time.Second)
	w.Step(dt)
	if w.CanLoot(1, 7) != nil {
		t.Fatal("emptied corpse should stay visible for 60 s")
	}
	clock = clock.Add(time.Second)
	w.Step(dt)
	if w.CanLoot(1, 7) == nil {
		t.Fatal("emptied corpse should be gone after 60 s")
	}
}

func TestCastingRootsAndDeadComeBackDead(t *testing.T) {
	w := duelAt(t, 0, 0)
	if err := w.Cast(1, "e", Point{1, 0}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := w.MoveTo(1, Point{5, 5}, false, t0); !errors.Is(err, ErrBusy) {
		t.Fatalf("moves are rooted while acting: %v", err)
	}
	w.Spawn(4, "亡者", Point{0, 5}, true, nil)
	if v, _ := w.Get(4); !v.Dead {
		t.Fatal("a character that left dead must come back dead")
	}
	if p, dead, ok := w.Despawn(4); !ok || !dead || p != (Point{0, 5}) {
		t.Fatalf("despawn %v %v %v", p, dead, ok)
	}
}

func TestSelfViewCarriesCooldownsAndRanges(t *testing.T) {
	w := duelAt(t, 0, 0)
	if err := w.Cast(1, "q", Point{1, 0}, 0); err != nil {
		t.Fatal(err)
	}
	w.Step(dt)
	v := w.selfView(w.ents[1])
	if v.Ranges["basic"] != 2 || v.Ranges["q"] != 3 {
		t.Fatalf("armed basic reaches 2 m and 穿雨 3 m: %+v", v.Ranges)
	}
	if cd := v.Cooldowns["q"]; cd < 4.9 || cd > 5 {
		t.Fatalf("q cooldown %.2f, want just under 5 s", cd)
	}
	if v.Cooldowns["e"] != 0 {
		t.Fatal("unused slot has no cooldown")
	}
}
