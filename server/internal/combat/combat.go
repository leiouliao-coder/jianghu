// Package combat is the deterministic combat timeline (move table v0.35,
// baseline §3.2). Time is counted in integer ticks of 0.05 s, the grid every
// move-table event sits on, so there is no float drift in timing.
//
// Each tick resolves in a fixed order: dash movement, then controls that
// take effect now (a stun interrupts actions whose effect is still in the
// future), then every damage hit of this tick applied together, then deaths.
// Effects already applied are never undone, so both sides may die together.
package combat

import (
	"errors"
	"math"
	"sort"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

type Tick int64

// Errors returned by Cast and Swap; none of them charge anything.
var (
	ErrDead        = errors.New("combat: caster is dead")
	ErrStunned     = errors.New("combat: caster is stunned")
	ErrBusy        = errors.New("combat: another action is in progress")
	ErrCooldown    = errors.New("combat: skill on cooldown")
	ErrResources   = errors.New("combat: not enough internal force or stamina")
	ErrNoSkill     = errors.New("combat: no skill in that slot")
	ErrBadTarget   = errors.New("combat: target is not a legal target")
	ErrOutOfRange  = errors.New("combat: target out of range")
	ErrNoDirection = errors.New("combat: direction required")
)

type Vec struct{ X, Z float64 }

func (a Vec) Sub(b Vec) Vec       { return Vec{a.X - b.X, a.Z - b.Z} }
func (a Vec) Add(b Vec) Vec       { return Vec{a.X + b.X, a.Z + b.Z} }
func (a Vec) Scale(f float64) Vec { return Vec{a.X * f, a.Z * f} }
func (a Vec) Dot(b Vec) float64   { return a.X*b.X + a.Z*b.Z }
func (a Vec) Len() float64        { return math.Hypot(a.X, a.Z) }
func (a Vec) Dist(b Vec) float64  { return a.Sub(b).Len() }
func (a Vec) Norm() (Vec, bool) {
	l := a.Len()
	if l < 1e-9 {
		return Vec{}, false
	}
	return a.Scale(1 / l), true
}

// Skill is a gamedata.Skill with times converted to ticks.
type Skill struct {
	gamedata.Skill
	Slot                     string
	windup, active, recovery Tick
	cooldown, stun           Tick
}

// effectAt is when an instant skill hits: after windup, plus any dash.
func (s *Skill) effectAt() Tick { return s.windup + s.active }
func (s *Skill) total() Tick    { return s.windup + s.active + s.recovery }

type Kind int

const (
	Player Kind = iota
	Monster
)

type action struct {
	skill     *Skill
	start     Tick
	dir       Vec
	target    int64
	length    float64 // rect length after lightness linkage
	slowOnHit bool
	parryUsed bool
	dashFrom  Vec
	dashDist  float64
}

// Fighter is one combatant. The world owns Pos and moves it while the
// fighter is not acting; combat moves it only during dashes.
type Fighter struct {
	ID                  int64
	Kind                Kind
	Pos                 Vec
	Facing              Vec
	HP, MP, SP          int
	MaxHP, MaxMP, MaxSP int
	Preset              string
	Skills              map[string]*Skill // slot -> skill for monsters; players use presets
	Dead                bool

	act              *action
	stunnedUntil     Tick
	stunImmuneUntil  Tick
	slowUntil        Tick
	slowCenti        int
	readyAt          map[string]Tick // slot -> tick; shared by both presets
	bonusUntil       Tick            // one layer of next-basic bonus damage
	lightUntil       Tick
	lightFrom        string // skill id of the lightness move that granted it
	swapUntil        Tick
	lastCombat       Tick
	lastPvP          Tick
	inCombatEver     bool
	regenAcc         [3]int // thousandths of HP, MP, SP
	gauge            []Tick // recent stun times, for control gauges
	gaugeImmuneUntil Tick
	controlGauge     *gamedata.MonsterKind
}

// Event is one resolved outcome, for logs, snapshots and tests.
type Event struct {
	Tick   Tick   `json:"tick"`
	Kind   string `json:"kind"` // cast, hit, miss, parry, stun, interrupt, refund, death
	Actor  int64  `json:"actor"`
	Target int64  `json:"target,omitempty"`
	Skill  string `json:"skill,omitempty"`
	Value  int    `json:"value,omitempty"`
}

// Rules decides whether attacker may harm target (zone permissions);
// Walkable bounds dashes. Both may be nil in tests.
type Rules interface {
	CanHarm(attacker, target *Fighter) bool
}

type Engine struct {
	Now      Tick
	data     *gamedata.Data
	cfg      gamedata.Combat
	skills   map[string]*Skill
	fighters map[int64]*Fighter
	Rules    Rules
	Walkable func(Vec) bool
	Regen    bool
	events   []Event
}

func Ticks(sec float64, tickSec float64) Tick { return Tick(math.Round(sec / tickSec)) }

func New(d *gamedata.Data) *Engine {
	e := &Engine{data: d, cfg: d.Combat, skills: map[string]*Skill{}, fighters: map[int64]*Fighter{}, Regen: true}
	for id, s := range d.Skills {
		e.skills[id] = e.convert(s)
	}
	for _, slots := range d.Combat.Presets {
		for slot, id := range slots {
			e.skills[id].Slot = slot
		}
	}
	return e
}

func (e *Engine) ticks(sec float64) Tick { return Ticks(sec, e.cfg.TickSeconds) }

func (e *Engine) convert(s gamedata.Skill) *Skill {
	return &Skill{Skill: s, windup: e.ticks(s.Windup), active: e.ticks(s.Active), recovery: e.ticks(s.Recovery),
		cooldown: e.ticks(s.Cooldown), stun: e.ticks(s.Stun)}
}

// NewPlayer makes a player fighter with the move table's starting resources.
func (e *Engine) NewPlayer(id int64, pos, facing Vec, preset string) *Fighter {
	in := e.cfg.Initial
	return &Fighter{ID: id, Kind: Player, Pos: pos, Facing: facing, HP: in.HP, MP: in.MP, SP: in.SP,
		MaxHP: in.HP, MaxMP: in.MP, MaxSP: in.SP, Preset: preset, readyAt: map[string]Tick{}}
}

// NewMonster makes a monster whose only skill is its basic attack.
func (e *Engine) NewMonster(id int64, kind gamedata.MonsterKind, pos Vec) *Fighter {
	a := kind.Attack
	s := e.convert(gamedata.Skill{ID: "monster_attack", Damage: a.Damage, Windup: a.Windup, Recovery: a.Recovery,
		Cooldown: a.Cooldown, Shape: "target", Range: a.Range, Parryable: a.Parryable})
	s.Slot = "basic"
	f := &Fighter{ID: id, Kind: Monster, Pos: pos, Facing: Vec{1, 0}, HP: kind.HP, MaxHP: kind.HP,
		Skills: map[string]*Skill{"basic": s}, readyAt: map[string]Tick{}}
	if kind.ControlGauge != nil {
		k := kind
		f.controlGauge = &k
	}
	return f
}

// MonsterKind returns a monster template from combat.json.
func (e *Engine) MonsterKind(id string) gamedata.MonsterKind { return e.cfg.Monsters[id] }

func (e *Engine) Add(f *Fighter)                           { e.fighters[f.ID] = f }
func (e *Engine) Remove(id int64)                          { delete(e.fighters, id) }
func (e *Engine) Fighter(id int64) *Fighter                { return e.fighters[id] }
func (e *Engine) Events() []Event                          { return e.events }
func (e *Engine) DrainEvents() []Event                     { ev := e.events; e.events = nil; return ev }
func (e *Engine) log(ev Event)                             { ev.Tick = e.Now; e.events = append(e.events, ev) }
func (f *Fighter) Busy() bool                              { return f.act != nil }
func (f *Fighter) Stunned(now Tick) bool                   { return now < f.stunnedUntil }
func (f *Fighter) CooldownLeft(slot string, now Tick) Tick { return max(0, f.readyAt[slot]-now) }

// ActionSkill names the skill in progress, or "".
func (f *Fighter) ActionSkill() string {
	if f.act == nil {
		return ""
	}
	return f.act.skill.ID
}

// CanMove reports whether the world may move this fighter now.
func (f *Fighter) CanMove(now Tick) bool { return !f.Dead && f.act == nil && !f.Stunned(now) }

// SpeedCenti is the walk speed factor after slows, floored per baseline §3.2.
func (e *Engine) SpeedCenti(f *Fighter) int {
	if e.Now < f.slowUntil {
		return max(100-f.slowCenti, e.cfg.Control.SlowFloorCenti)
	}
	return 100
}

// InCombat reports the fighter's combat state: PvE leaves combat after 15 s
// and PvP after 30 s without effective hostile actions (baseline §3.2).
func (e *Engine) InCombat(f *Fighter, pveSec, pvpSec float64) bool {
	if !f.inCombatEver {
		return false
	}
	if f.lastPvP > 0 && e.Now-f.lastPvP < e.ticks(pvpSec) {
		return true
	}
	return e.Now-f.lastCombat < e.ticks(pveSec)
}

func (e *Engine) skillFor(f *Fighter, slot string) *Skill {
	if f.Kind == Monster {
		return f.Skills[slot]
	}
	if id, ok := e.cfg.Presets[f.Preset][slot]; ok {
		return e.skills[id]
	}
	return nil
}

// Cast starts slot's skill now. A refused cast charges nothing and starts
// no cooldown; a legal start charges resources and starts the cooldown even
// if the skill later misses or is interrupted.
func (e *Engine) Cast(id int64, slot string, dir Vec, target int64) error {
	f := e.fighters[id]
	switch {
	case f == nil || f.Dead:
		return ErrDead
	case f.Stunned(e.Now):
		return ErrStunned
	case f.act != nil || e.Now < f.swapUntil:
		return ErrBusy
	}
	s := e.skillFor(f, slot)
	if s == nil {
		return ErrNoSkill
	}
	if e.Now < f.readyAt[slot] {
		return ErrCooldown
	}
	if f.MP < s.MP || f.SP < s.SP {
		return ErrResources
	}
	a := &action{skill: s, start: e.Now, target: target, length: s.Length}
	if s.Shape == "target" {
		t := e.fighters[target]
		if t == nil || t.Dead || t == f || (e.Rules != nil && !e.Rules.CanHarm(f, t)) {
			return ErrBadTarget
		}
		if f.Pos.Dist(t.Pos) > s.Range {
			return ErrOutOfRange
		}
		dir = t.Pos.Sub(f.Pos)
	}
	n, ok := dir.Norm()
	if !ok {
		if s.Shape != "target" {
			return ErrNoDirection
		}
		n = f.Facing
	}
	a.dir = n
	if s.Shape == "dash" {
		a.dashFrom, a.dashDist = f.Pos, math.Min(dir.Len(), s.Range)
		if a.dashDist < 1e-9 {
			a.dashDist = s.Range
		}
	} else if s.Dash > 0 {
		a.dashFrom, a.dashDist = f.Pos, s.Dash
	}
	// Lightness linkage: consumed by the next Q started within its window.
	if slot == "q" && e.Now < f.lightUntil {
		switch f.lightFrom {
		case "sword_l":
			a.length = e.cfg.Passives.Lightness.SwordQLength
		case "fist_l":
			a.slowOnHit = true
		}
		f.lightUntil = 0
	}
	f.MP -= s.MP
	f.SP -= s.SP
	f.readyAt[slot] = e.Now + s.cooldown
	f.Facing = n
	f.act = a
	e.log(Event{Kind: "cast", Actor: f.ID, Target: target, Skill: s.ID})
	return nil
}

// Swap starts the 1 s switch between armed and unarmed presets; cooldowns
// are per slot and so shared by both presets (the longer remaining applies).
func (e *Engine) Swap(id int64) error {
	f := e.fighters[id]
	switch {
	case f == nil || f.Dead || f.Kind != Player:
		return ErrDead
	case f.Stunned(e.Now):
		return ErrStunned
	case f.act != nil || e.Now < f.swapUntil:
		return ErrBusy
	}
	f.swapUntil = e.Now + e.ticks(e.cfg.Control.WeaponSwapSeconds)
	return nil
}

type hit struct {
	from   *Fighter
	to     *Fighter
	act    *action
	damage int
}

// Step advances one tick and resolves everything due at the new time.
func (e *Engine) Step() {
	e.Now++
	ids := make([]int64, 0, len(e.fighters))
	for id := range e.fighters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// 1. Dashes and swaps.
	for _, id := range ids {
		f := e.fighters[id]
		if f.swapUntil != 0 && e.Now >= f.swapUntil && !f.Dead {
			f.Preset = map[string]string{"armed": "unarmed", "unarmed": "armed"}[f.Preset]
			f.swapUntil = 0
		}
		if a := f.act; a != nil && a.dashDist > 0 && a.skill.active > 0 {
			el := e.Now - a.start - a.skill.windup
			if el > 0 && el <= a.skill.active {
				next := a.dashFrom.Add(a.dir.Scale(a.dashDist * float64(el) / float64(a.skill.active)))
				// Blocked dashes stop short: never through walls.
				if e.Walkable == nil || e.Walkable(next) {
					f.Pos = next
				} else {
					a.dashDist = 0
				}
			}
		}
	}

	// 2. Collect the hits that take effect now.
	var hits []hit
	for _, id := range ids {
		f := e.fighters[id]
		a := f.act
		if a == nil || f.Dead || e.Now != a.start+a.skill.effectAt() {
			continue
		}
		s := a.skill
		if s.Shape == "parry" || s.Shape == "dash" {
			continue
		}
		targets := e.targets(f, a, ids)
		if len(targets) == 0 {
			e.log(Event{Kind: "miss", Actor: f.ID, Skill: s.ID})
		}
		for _, t := range targets {
			hits = append(hits, hit{from: f, to: t, act: a, damage: s.Damage})
		}
	}

	// A parry reacts to the hit itself, so it is decided before the hit's
	// control or damage applies; a parried hit does nothing at all.
	blocked := make([]bool, len(hits))
	for i := range hits {
		blocked[i] = e.parried(&hits[i])
	}

	// 3. Controls first: a stun ends actions still in progress.
	for i, h := range hits {
		if !blocked[i] && h.act.skill.stun > 0 {
			e.stun(h.from, h.to, h.act.skill)
		}
	}

	// 4. Damage batch, applied together. Hits already collected for this
	// tick land even if a control above ended their attacker's action.
	for i := range hits {
		h := &hits[i]
		if blocked[i] {
			continue
		}
		dmg := h.damage
		if h.act.skill.Slot == "basic" && h.from.Kind == Player && e.Now < h.from.bonusUntil {
			dmg += e.cfg.Passives.NextBasicBonus.Damage
			h.from.bonusUntil = 0
		}
		h.to.HP -= dmg
		e.markCombat(h.from, h.to)
		if h.act.slowOnHit {
			lt := e.cfg.Passives.Lightness
			h.to.slowCenti = lt.FistQSlowCenti
			h.to.slowUntil = e.Now + e.ticks(lt.FistQSlowSeconds)
		}
		e.log(Event{Kind: "hit", Actor: h.from.ID, Target: h.to.ID, Skill: h.act.skill.ID, Value: dmg})
	}

	// 5. Deaths, then action completion, buffs and regen.
	for _, id := range ids {
		f := e.fighters[id]
		if !f.Dead && f.HP <= 0 {
			f.HP, f.Dead, f.act, f.swapUntil = 0, true, nil, 0
			e.log(Event{Kind: "death", Actor: f.ID})
		}
	}
	for _, id := range ids {
		f := e.fighters[id]
		if a := f.act; a != nil && e.Now >= a.start+a.skill.total() {
			if a.skill.Shape == "dash" {
				f.lightFrom = a.skill.ID
				f.lightUntil = e.Now + e.ticks(e.cfg.Passives.Lightness.Seconds)
			}
			f.act = nil
		}
		e.regen(f)
	}
}

// targets returns who an action's geometry covers at its effect time,
// using positions now and the direction locked at the start.
func (e *Engine) targets(f *Fighter, a *action, ids []int64) []*Fighter {
	s := a.skill
	var out []*Fighter
	if s.Shape == "target" {
		t := e.fighters[a.target]
		// Re-checked at effect time: a target that left range is missed.
		if t != nil && !t.Dead && f.Pos.Dist(t.Pos) <= s.Range && e.harm(f, t) {
			out = append(out, t)
		}
		return out
	}
	for _, id := range ids {
		t := e.fighters[id]
		if t == f || t.Dead || !e.harm(f, t) {
			continue
		}
		rel := t.Pos.Sub(f.Pos)
		switch s.Shape {
		case "rect":
			along := rel.Dot(a.dir)
			side := math.Abs(rel.Dot(Vec{-a.dir.Z, a.dir.X}))
			if along >= 0 && along <= a.length+1e-9 && side <= s.HalfWidth+1e-9 {
				out = append(out, t)
			}
		case "sector":
			if inSector(rel, a.dir, s.Range, s.Arc) {
				out = append(out, t)
			}
		}
	}
	return out
}

func (e *Engine) harm(f, t *Fighter) bool { return e.Rules == nil || e.Rules.CanHarm(f, t) }

func inSector(rel, dir Vec, rng, arcDeg float64) bool {
	d := rel.Len()
	if d > rng+1e-9 {
		return false
	}
	if d < 1e-9 {
		return true
	}
	return rel.Scale(1/d).Dot(dir) >= math.Cos(arcDeg/2*math.Pi/180)-1e-9
}

// parried reports whether the target's active parry window stops this hit:
// first qualifying frontal direct melee attack within range, once per parry.
func (e *Engine) parried(h *hit) bool {
	t := h.to
	a := t.act
	if a == nil || a.skill.Shape != "parry" || a.parryUsed || !h.act.skill.Parryable {
		return false
	}
	open := a.start + a.skill.windup
	if e.Now < open || e.Now > open+a.skill.active {
		return false
	}
	if !inSector(h.from.Pos.Sub(t.Pos), a.dir, a.skill.Range, a.skill.Arc) {
		return false
	}
	a.parryUsed = true
	p := e.cfg.Passives
	t.SP = min(t.MaxSP, t.SP+p.InternalRefund.SwordParrySP)
	t.bonusUntil = e.Now + e.ticks(p.NextBasicBonus.Seconds)
	e.markCombat(h.from, t)
	e.log(Event{Kind: "parry", Actor: t.ID, Target: h.from.ID, Skill: h.act.skill.ID})
	e.log(Event{Kind: "refund", Actor: t.ID, Value: p.InternalRefund.SwordParrySP})
	return true
}

// stun applies a stun unless the target is immune; it interrupts any action
// whose effect has not happened yet and ends recovery early. A stun that
// interrupts a windup refunds the attacker's internal force (守一诀) and
// grants the next-basic bonus (乘隙).
func (e *Engine) stun(from, t *Fighter, s *Skill) {
	if e.Now < t.stunImmuneUntil || e.Now < t.gaugeImmuneUntil {
		e.log(Event{Kind: "stun_immune", Actor: from.ID, Target: t.ID, Skill: s.ID})
		return
	}
	dur := s.stun
	if g := t.controlGauge; g != nil && g.ControlGauge != nil {
		cg := g.ControlGauge
		dur = dur * Tick(cg.StunScaleCenti) / 100
		window := e.ticks(cg.WindowSeconds)
		recent := t.gauge[:0]
		for _, at := range t.gauge {
			if e.Now-at < window {
				recent = append(recent, at)
			}
		}
		t.gauge = append(recent, e.Now)
		if len(t.gauge) >= cg.StunsBeforeImmune {
			t.gaugeImmuneUntil = e.Now + dur + e.ticks(cg.ImmunitySeconds)
			t.gauge = nil
		}
	}
	// Only a windup counts as interrupted (the refund and bonus trigger);
	// later phases simply end, and effects already applied stand.
	interrupted := false
	if a := t.act; a != nil {
		interrupted = e.Now < a.start+a.skill.windup
		t.act = nil
	}
	t.swapUntil = 0
	t.stunnedUntil = e.Now + dur
	t.stunImmuneUntil = t.stunnedUntil + e.ticks(e.cfg.Control.StunImmunitySeconds)
	e.log(Event{Kind: "stun", Actor: from.ID, Target: t.ID, Skill: s.ID, Value: int(dur)})
	if interrupted {
		p := e.cfg.Passives
		from.MP = min(from.MaxMP, from.MP+p.InternalRefund.FistInterruptMP)
		from.bonusUntil = e.Now + e.ticks(p.NextBasicBonus.Seconds)
		e.log(Event{Kind: "interrupt", Actor: from.ID, Target: t.ID})
		e.log(Event{Kind: "refund", Actor: from.ID, Value: p.InternalRefund.FistInterruptMP})
	}
}

func (e *Engine) markCombat(a, b *Fighter) {
	for _, f := range []*Fighter{a, b} {
		f.lastCombat, f.inCombatEver = e.Now, true
		if a.Kind == Player && b.Kind == Player {
			f.lastPvP = e.Now
		}
	}
}

// regen restores resources only out of combat (authored test parameters).
func (e *Engine) regen(f *Fighter) {
	if !e.Regen || f.Dead || e.InCombat(f, 15, 30) {
		return
	}
	r := e.cfg.Regen
	ms := int(e.cfg.TickSeconds * 1000)
	for i, per := range []int{r.HPPerSecond, r.MPPerSecond, r.SPPerSecond} {
		f.regenAcc[i] += per * ms
		gain := f.regenAcc[i] / 1000
		f.regenAcc[i] %= 1000
		switch i {
		case 0:
			f.HP = min(f.MaxHP, f.HP+gain)
		case 1:
			f.MP = min(f.MaxMP, f.MP+gain)
		case 2:
			f.SP = min(f.MaxSP, f.SP+gain)
		}
	}
}
