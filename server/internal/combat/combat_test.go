package combat

import (
	"errors"
	"testing"
)

type duel struct {
	t     *testing.T
	e     *Engine
	sword *Fighter
	fist  *Fighter
}

// newDuel puts the sword at the origin facing +x and the fist at x=gap
// facing -x, with regen off unless a test turns it on.
func newDuel(t *testing.T, gap float64) *duel {
	t.Helper()
	e := New(loadData(t))
	e.Regen = false
	d := &duel{t: t, e: e, sword: e.NewPlayer(1, Vec{0, 0}, Vec{1, 0}, "armed"), fist: e.NewPlayer(2, Vec{gap, 0}, Vec{-1, 0}, "unarmed")}
	e.Add(d.sword)
	e.Add(d.fist)
	return d
}

func (d *duel) cast(f *Fighter, slot string, dir Vec, target int64) {
	d.t.Helper()
	if err := d.e.Cast(f.ID, slot, dir, target); err != nil {
		d.t.Fatalf("cast %s: %v", slot, err)
	}
}

func (d *duel) run(seconds float64) {
	for range Ticks(seconds, 0.05) {
		d.e.Step()
	}
}

func (d *duel) count(kind string) int {
	n := 0
	for _, ev := range d.e.Events() {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func TestRefusedCastsChargeNothing(t *testing.T) {
	d := newDuel(t, 5)
	s := d.sword
	check := func(name string, err, want error) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Fatalf("%s: err=%v want %v", name, err, want)
		}
		if s.MP != 100 || s.SP != 100 || s.CooldownLeft("e", d.e.Now) != 0 || s.Busy() {
			t.Fatalf("%s charged something: %d/%d", name, s.MP, s.SP)
		}
	}
	check("out of range", d.e.Cast(1, "basic", Vec{}, 2), ErrOutOfRange)
	check("no target", d.e.Cast(1, "basic", Vec{}, 99), ErrBadTarget)
	check("self target", d.e.Cast(1, "basic", Vec{}, 1), ErrBadTarget)
	check("no direction", d.e.Cast(1, "e", Vec{}, 0), ErrNoDirection)
	check("no slot", d.e.Cast(1, "x", Vec{1, 0}, 0), ErrNoSkill)
	s.MP = 10
	if err := d.e.Cast(1, "e", Vec{1, 0}, 0); !errors.Is(err, ErrResources) || s.MP != 10 {
		t.Fatalf("resources: %v mp=%d", err, s.MP)
	}
	s.MP = 100
	d.cast(s, "e", Vec{1, 0}, 0) // legal: charges 15/15 and starts the 10 s cooldown
	if err := d.e.Cast(1, "q", Vec{1, 0}, 0); !errors.Is(err, ErrBusy) || s.MP != 85 {
		t.Fatalf("busy: %v", err)
	}
	d.run(1.5)
	if err := d.e.Cast(1, "e", Vec{1, 0}, 0); !errors.Is(err, ErrCooldown) || s.MP != 85 {
		t.Fatalf("cooldown: %v", err)
	}
	if d.count("miss") != 1 {
		t.Fatal("the legal sword_e should have missed (target 5 m away) yet kept its cost")
	}
}

func TestStunImmunityAndRecovery(t *testing.T) {
	d := newDuel(t, 1.2)
	d.cast(d.fist, "w", d.fist.Facing, 0)
	d.run(0.15) // effect at 0.15: stun 0.45 s, then 2 s immunity
	if !d.sword.Stunned(d.e.Now) {
		t.Fatal("first 截脉 must stun")
	}
	if err := d.e.Cast(1, "q", Vec{1, 0}, 0); !errors.Is(err, ErrStunned) {
		t.Fatalf("stunned caster: %v", err)
	}
	d.run(0.5)
	if d.sword.Stunned(d.e.Now) {
		t.Fatal("stun lasts 0.45 s only")
	}
	d.fist.readyAt["w"] = 0 // test the immunity, not the cooldown
	d.run(0.4)
	d.cast(d.fist, "w", d.fist.Facing, 0)
	d.run(0.15)
	if d.sword.Stunned(d.e.Now) || d.count("stun_immune") != 1 {
		t.Fatal("a second stun inside the 2 s immunity must not stun")
	}
	if d.sword.HP != 1000-80-80 {
		t.Fatalf("immunity blocks the stun, not the damage: hp=%d", d.sword.HP)
	}
}

func TestBothDieOnTheSameTick(t *testing.T) {
	d := newDuel(t, 1.2)
	d.sword.HP, d.fist.HP = 50, 50
	d.cast(d.sword, "basic", Vec{}, 2) // effect at 0.25
	d.e.Step()
	d.cast(d.fist, "basic", Vec{}, 1) // started 0.05 later, windup 0.20: also hits at 0.25
	d.run(0.3)
	if !d.sword.Dead || !d.fist.Dead || d.count("death") != 2 {
		t.Fatalf("simultaneous hits must kill both: %+v", d.e.Events())
	}
}

func TestDeathCancelsPendingAction(t *testing.T) {
	d := newDuel(t, 1.2)
	d.sword.HP = 50
	d.cast(d.sword, "e", Vec{1, 0}, 0) // would hit at 0.80
	d.cast(d.fist, "basic", Vec{}, 1)  // kills at 0.20
	d.run(1.5)
	if !d.sword.Dead || d.fist.HP != 1000 {
		t.Fatalf("a dead caster's unfired action must not land: fist hp %d", d.fist.HP)
	}
}

func TestParryGrantsNextBasicBonus(t *testing.T) {
	d := newDuel(t, 1.4)
	d.cast(d.sword, "w", Vec{1, 0}, 0)
	d.run(0.15) // parry window open
	d.cast(d.fist, "basic", Vec{}, 1)
	d.run(1.0)
	if d.sword.HP != 1000 || d.count("parry") != 1 || d.sword.SP != 100-12+4 {
		t.Fatalf("parry: hp=%d sp=%d", d.sword.HP, d.sword.SP)
	}
	d.cast(d.sword, "basic", Vec{}, 2)
	d.run(0.8)
	d.cast(d.sword, "basic", Vec{}, 2)
	d.run(0.8)
	if d.fist.HP != 1000-80-60 {
		t.Fatalf("留锋: first basic +20 then consumed, fist hp=%d want %d", d.fist.HP, 1000-80-60)
	}
}

func TestParryOnlyCoversFrontAndWindow(t *testing.T) {
	d := newDuel(t, 1.4)
	d.cast(d.sword, "w", Vec{-1, 0}, 0) // facing away
	d.run(0.15)
	d.cast(d.fist, "basic", Vec{}, 1)
	d.run(1)
	if d.sword.HP != 1000-55 {
		t.Fatalf("a parry facing away must not block: hp=%d", d.sword.HP)
	}
	d2 := newDuel(t, 1.4)
	d2.cast(d2.sword, "w", Vec{1, 0}, 0)
	d2.run(0.7) // window 0.15–0.55 is over
	d2.cast(d2.fist, "basic", Vec{}, 1)
	d2.run(1)
	if d2.sword.HP != 1000-55 {
		t.Fatalf("a hit after the window must land: hp=%d", d2.sword.HP)
	}
}

func TestTargetLeavingRangeIsMissed(t *testing.T) {
	d := newDuel(t, 1.8)
	d.cast(d.sword, "basic", Vec{}, 2)
	d.e.Step()
	d.fist.Pos.X = 3 // stepped out before the 0.25 s effect
	d.run(1)
	if d.fist.HP != 1000 || d.count("miss") != 1 {
		t.Fatal("range is re-checked at the effect time")
	}
}

func TestDirectionIsLockedAtStart(t *testing.T) {
	d := newDuel(t, 2)
	d.cast(d.sword, "q", Vec{1, 0}, 0)
	d.sword.Facing = Vec{-1, 0} // turning later changes nothing
	d.fist.Pos = Vec{-2, 0}     // the fist moves behind
	d.run(1)
	if d.fist.HP != 1000 {
		t.Fatal("穿雨 hits along the direction locked at the start")
	}
}

func TestSwapTakesOneSecondAndSharesCooldowns(t *testing.T) {
	d := newDuel(t, 5)
	d.cast(d.sword, "q", Vec{1, 0}, 0)
	d.run(1)
	if err := d.e.Swap(1); err != nil {
		t.Fatal(err)
	}
	if err := d.e.Cast(1, "e", Vec{1, 0}, 0); !errors.Is(err, ErrBusy) {
		t.Fatal("no attacks while swapping")
	}
	d.run(1)
	if d.sword.Preset != "unarmed" {
		t.Fatal("swap completes after 1 s")
	}
	if err := d.e.Cast(1, "q", Vec{1, 0}, 0); !errors.Is(err, ErrCooldown) {
		t.Fatalf("Q slot cooldown is shared across presets: %v", err)
	}
}

func TestRegenOnlyOutOfCombat(t *testing.T) {
	// Player vs player: combat lasts 30 s after the last hit.
	d := newDuel(t, 1.2)
	d.e.Regen = true
	d.cast(d.fist, "basic", Vec{}, 1)
	d.run(1)
	hp := d.sword.HP
	d.run(25)
	if d.sword.HP != hp || !d.e.InCombat(d.sword) {
		t.Fatal("PvP: no regen within 30 s of the last hit")
	}
	d.run(6)
	if d.sword.HP <= hp {
		t.Fatal("regen resumes after leaving PvP combat")
	}

	// Player vs monster: 15 s.
	e := New(loadData(t))
	p := e.NewPlayer(1, Vec{0, 0}, Vec{1, 0}, "armed")
	m := e.NewMonster(2, e.data.Combat.Monsters["bandit"], Vec{1.5, 0})
	e.Add(p)
	e.Add(m)
	if err := e.Cast(2, "basic", Vec{}, 1); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		e.Step()
	}
	hp = p.HP
	for range Ticks(14, 0.05) {
		e.Step()
	}
	if p.HP != hp {
		t.Fatal("PvE: no regen within 15 s")
	}
	for range Ticks(2, 0.05) {
		e.Step()
	}
	if p.HP <= hp {
		t.Fatal("PvE: regen resumes after 15 s")
	}
}

func TestBossControlGauge(t *testing.T) {
	d := newDuel(t, 1.2)
	boss := d.e.NewMonster(3, d.e.data.Combat.Monsters["boss"], Vec{-1.2, 0})
	d.e.Add(boss)
	stunAt := func() Tick {
		d.fist.readyAt["w"] = 0
		d.fist.Pos, d.fist.Facing = Vec{-2.4, 0}, Vec{1, 0}
		d.cast(d.fist, "w", Vec{1, 0}, 0)
		d.run(0.15)
		return boss.stunnedUntil - d.e.Now
	}
	if dur := stunAt(); dur != 4 { // 0.45 s halved -> 0.2 s (4 ticks)
		t.Fatalf("boss stun %d ticks, want 4", dur)
	}
	d.run(2.5) // past the per-target stun immunity
	stunAt()
	d.run(2.5)
	stunAt() // third stun inside the 10 s window -> gauge immunity
	d.run(2.5)
	if dur := stunAt(); dur > 0 {
		t.Fatal("gauge immunity must block the fourth stun")
	}
}

func TestDashStopsAtWalls(t *testing.T) {
	d := newDuel(t, 9)
	d.e.Walkable = func(p Vec) bool { return p.X < 1.6 }
	d.cast(d.sword, "l", Vec{3, 0}, 0)
	d.run(1)
	if d.sword.Pos.X >= 1.6 || d.sword.Pos.X < 1 {
		t.Fatalf("dash ended at %.2f, must stop before the wall at 1.6", d.sword.Pos.X)
	}
}

func TestLightnessLinkageExpires(t *testing.T) {
	d := newDuel(t, 6.3)
	d.cast(d.sword, "l", Vec{3, 0}, 0)
	d.run(0.6 + 2.5) // linkage lasted until 2.6 s
	d.cast(d.sword, "q", Vec{1, 0}, 0)
	d.run(1)
	if d.fist.HP != 1000 {
		t.Fatal("expired linkage must not lengthen 穿雨")
	}
}

func TestFistLightnessSlowsAndFloor(t *testing.T) {
	d := newDuel(t, 4)
	d.cast(d.fist, "l", Vec{-2, 0}, 0)
	d.run(0.7)
	d.cast(d.fist, "q", Vec{-1, 0}, 0)
	d.run(0.45)
	if got := d.e.SpeedCenti(d.sword); got != 80 {
		t.Fatalf("踏石 after 贴岩 slows 20%%: speed %d", got)
	}
	d.run(1)
	if got := d.e.SpeedCenti(d.sword); got != 100 {
		t.Fatalf("slow lasts 0.8 s: speed %d", got)
	}
	d.sword.slowCenti, d.sword.slowUntil = 90, d.e.Now+10
	if got := d.e.SpeedCenti(d.sword); got != 40 {
		t.Fatalf("slow floor is 40%%: speed %d", got)
	}
}

type denyAll struct{}

func (denyAll) CanHarm(_, _ *Fighter) bool { return false }

func TestRulesForbidHarm(t *testing.T) {
	d := newDuel(t, 1.2)
	d.e.Rules = denyAll{}
	if err := d.e.Cast(1, "basic", Vec{}, 2); !errors.Is(err, ErrBadTarget) {
		t.Fatalf("targeted attack in a no-harm zone: %v", err)
	}
	d.cast(d.sword, "e", Vec{1, 0}, 0) // area attacks may start, but skip protected targets
	d.run(1.5)
	if d.fist.HP != 1000 {
		t.Fatal("protected target took damage")
	}
}
