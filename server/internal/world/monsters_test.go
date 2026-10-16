package world

import (
	"testing"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

func monsterWorld(t *testing.T) *World {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	w := New(d)
	w.SpawnMonsters(d)
	return w
}

// bandit is the first spawn: a mining-area bandit at (-85, -20).
const bandit = int64(-1)

func TestBanditAggroesChasesAndAttacks(t *testing.T) {
	w := monsterWorld(t)
	w.Spawn(1, "矿工", Point{-85, -10}, false, nil) // 10 m away: outside the 8 m aggro
	steps(w, 3)
	if v, _ := w.Get(1); v.HP != v.MaxHP {
		t.Fatal("a player outside aggro range was attacked")
	}
	w.Spawn(2, "冒失鬼", Point{-85, -14}, false, nil) // 6 m away
	steps(w, 6)
	v, _ := w.Get(2)
	if v.HP >= v.MaxHP {
		t.Fatalf("bandit should have chased and hit the near player: %+v", v)
	}
	if m, _ := w.Get(bandit); m.Kind != "monster" || m.Name != "矿区山贼" {
		t.Fatalf("monster view %+v", m)
	}
}

func TestMonstersLeavePlayersInTownAlone(t *testing.T) {
	w := monsterWorld(t)
	m := w.ents[bandit]
	m.pos, m.f.Pos = Point{-34, 0}, vec(Point{-34, 0}) // on the A-C road, near town
	m.mob.spawn = Point{-34, 0}
	w.Spawn(1, "镇民", Point{-28, 0}, false, nil) // just inside town, 6 m away
	steps(w, 5)
	if v, _ := w.Get(1); v.HP != v.MaxHP {
		t.Fatal("a player inside a service zone was attacked")
	}

	// A chase into town stops at the edge.
	w.Spawn(2, "逃跑者", Point{-38, 0}, false, nil)
	steps(w, 0.5)
	if _, err := w.MoveTo(2, Point{-10, 0}, false, t0); err != nil {
		t.Fatal(err)
	}
	steps(w, 10)
	for _, p := range []Point{w.ents[bandit].pos} {
		if a, _ := w.nav.Classify(p[0], p[1]); a.Zone == ZoneService {
			t.Fatalf("monster entered town at %v", p)
		}
	}
}

func TestLeashResetsAndHeals(t *testing.T) {
	w := monsterWorld(t)
	w.Spawn(1, "风筝手", Point{-85, -14}, false, nil)
	steps(w, 1)
	w.SetHP(bandit, 100)
	// Kite it east along the road, far past the 20 m leash.
	if _, err := w.MoveTo(1, Point{-40, 0}, false, t0); err != nil {
		t.Fatal(err)
	}
	// Check the moment it arrives home, before ordinary regen could explain
	// a full bar (regen only starts 15 s after the last exchange).
	m := w.ents[bandit]
	wasHoming := false
	for range int(25 / dt) {
		w.Step(dt)
		if m.mob.homing {
			wasHoming = true
		} else if wasHoming {
			break
		}
	}
	if !wasHoming || dist(m.pos, m.mob.spawn) > 1 {
		t.Fatalf("monster should have walked home, %0.1f m away (homing seen %v)", dist(m.pos, m.mob.spawn), wasHoming)
	}
	if v, _ := w.Get(bandit); v.HP != v.MaxHP {
		t.Fatalf("monster should heal on arriving home: %+v", v)
	}
}

// The edge guard: a chase whose shortest path runs through town must stop
// at the town edge instead of cutting through.
func TestChaseNeverCutsThroughTown(t *testing.T) {
	w := monsterWorld(t)
	m := w.ents[bandit]
	m.pos, m.f.Pos, m.mob.spawn = Point{-35, 0}, vec(Point{-35, 0}), Point{-35, 0}
	m.mob.kind.AggroRange, m.mob.kind.LeashRange = 100, 100
	w.Spawn(1, "诱饵", Point{0, 34}, false, nil) // on the A-E road, beyond town
	for range int(20 / dt) {
		w.Step(dt)
		if a, _ := w.nav.Classify(m.pos[0], m.pos[1]); a.Zone == ZoneService {
			t.Fatalf("monster cut through town at %v", m.pos)
		}
	}
}

func TestMonsterRespawns(t *testing.T) {
	w := monsterWorld(t)
	w.SetHP(bandit, 0)
	steps(w, 1)
	if v, _ := w.Get(bandit); !v.Dead {
		t.Fatal("monster should be dead")
	}
	steps(w, 58)
	if v, _ := w.Get(bandit); !v.Dead {
		t.Fatal("respawned before 60 s")
	}
	steps(w, 2)
	v, _ := w.Get(bandit)
	if v.Dead || v.HP != v.MaxHP || v.X != -85 || v.Z != -20 {
		t.Fatalf("respawn at the spawn with full health: %+v", v)
	}
	if _, ok := w.Positions()[bandit]; ok {
		t.Fatal("monsters must not be saved as characters")
	}
}

func TestPlayersStillCannotFightEachOtherInTheWild(t *testing.T) {
	w := monsterWorld(t)
	w.Spawn(1, "甲", Point{-100, 25}, false, nil)
	w.Spawn(2, "乙", Point{-98.8, 25}, false, nil)
	steps(w, 0.1)
	if err := w.Cast(1, "basic", Point{}, 2); err == nil {
		t.Fatal("player-vs-player outside risk areas must be refused")
	}
}
