// Package world is the authoritative world simulation. One loop owns every
// character's position and combat state; the block a character is in
// follows from its position, so it is always in exactly one block or road.
// Clients only send destinations and skill requests: the server plans the
// path, moves at the fixed walk speed and resolves combat, so a client
// cannot choose its own speed, position or damage.
//
// A single loop covers the six-block prototype. Splitting blocks into
// separate loops or processes is a scaling step for after load tests (G2).
package world

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/leiouliao-coder/jianghu/server/internal/combat"
	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

var (
	ErrNotInWorld  = errors.New("world: character not in world")
	ErrRateLimited = errors.New("world: too many move requests")
	ErrBusy        = errors.New("world: acting, stunned or dead")
	ErrNotDead     = errors.New("world: character is not dead")
	ErrDead        = errors.New("world: character is dead")
	ErrTooFar      = errors.New("world: too far away")
)

// maxPendingEvents bounds combat events kept for the next snapshot.
const maxPendingEvents = 256

// LootRange is how close a character must stand to loot a corpse (meters).
const LootRange = 3.0

// CorpseTTL is how long a corpse keeps its goods; EmptyCorpseLinger is how
// long an emptied corpse stays visible (world.json corpse, baseline §0.1).
const (
	CorpseTTL         = 30 * time.Minute
	EmptyCorpseLinger = 60 * time.Second
)

// Sink receives pushes for a connected character. It must not block.
type Sink interface {
	Push(typ string, payload any)
}

type entity struct {
	id        int64
	name      string
	pos       Point
	area      Area
	path      []Point // remaining waypoints, pos excluded
	sink      Sink    // nil while the owner is disconnected
	moveTimes []time.Time
	f         *combat.Fighter
}

type corpse struct {
	id      int64
	pos     Point
	owner   string
	expires time.Time
	emptyAt time.Time // zero while goods remain
}

// Hooks are called outside the world lock.
type Hooks struct {
	// OnDeath runs once per death of a character; it must move the carried
	// goods into a corpse and then call AddCorpse.
	OnDeath func(character int64, name string, at Point)
	// OnCorpseExpired must destroy what is left in the corpse.
	OnCorpseExpired func(corpse int64)
}

type World struct {
	nav   *Nav
	eng   *combat.Engine
	speed float64
	view  float64
	tickD time.Duration
	every int // ticks per snapshot
	limit int // move requests per second
	spawn Point
	safe  []Point // respawn points: one per service block
	Hooks Hooks
	Now   func() time.Time // wall clock for corpse timers; tests replace it

	mu      sync.Mutex
	ents    map[int64]*entity
	corpses map[int64]*corpse
	tick    uint64
	pending []combat.Event // combat events since the last snapshot
}

func New(d *gamedata.Data) *World {
	mv := d.Map.Movement
	w := &World{
		nav:     NewNav(d),
		eng:     combat.New(d),
		speed:   mv.WalkSpeed,
		view:    mv.ViewRadius,
		tickD:   time.Second / time.Duration(mv.TickHz),
		every:   mv.TickHz / mv.SnapshotHz,
		limit:   mv.MaxMoveRequestsPerSecond,
		spawn:   Point{d.Map.Spawn.X, d.Map.Spawn.Z},
		Now:     time.Now,
		ents:    map[int64]*entity{},
		corpses: map[int64]*corpse{},
	}
	w.eng.Rules = w
	w.eng.Walkable = func(v combat.Vec) bool { _, ok := w.nav.Classify(v.X, v.Z); return ok }
	for _, b := range d.Map.Blocks {
		if a, ok := w.nav.Classify((b.Rect[0]+b.Rect[2])/2, (b.Rect[1]+b.Rect[3])/2); ok && a.Zone == ZoneService {
			w.safe = append(w.safe, Point{(b.Rect[0] + b.Rect[2]) / 2, (b.Rect[1] + b.Rect[3]) / 2})
		}
	}
	return w
}

func (w *World) Nav() *Nav { return w.nav }

// CanHarm implements the zone rules (world.json zone_rules): nobody harms or
// is harmed inside a service zone; players harm players only when both
// stand in a risk area; monsters and players fight anywhere else.
func (w *World) CanHarm(a, b *combat.Fighter) bool {
	za, zb := w.zoneOf(a.ID), w.zoneOf(b.ID)
	if za == ZoneService || zb == ZoneService {
		return false
	}
	if a.Kind == combat.Player && b.Kind == combat.Player {
		return za == ZoneRisk && zb == ZoneRisk
	}
	return true
}

func (w *World) zoneOf(id int64) string {
	if e := w.ents[id]; e != nil {
		return e.area.Zone
	}
	return ""
}

// ClientMap is the geometry clients need to draw the graybox, sent with
// enter_world so the server's map.json stays the only source.
func ClientMap(d *gamedata.Data) map[string]any {
	names := map[string]string{}
	zones := map[string]string{}
	for _, b := range d.World.Blocks {
		names[b.ID], zones[b.ID] = b.Name, b.Zone
	}
	m := d.Map
	blocks := make([]map[string]any, 0, len(m.Blocks))
	for _, b := range m.Blocks {
		blocks = append(blocks, map[string]any{"id": b.ID, "name": names[b.ID], "zone": zones[b.ID], "rect": b.Rect})
	}
	return map[string]any{
		"bounds": m.Bounds, "blocks": blocks, "risk_areas": m.RiskAreas, "roads": m.Roads, "obstacles": m.Obstacles,
		"walk_speed": m.Movement.WalkSpeed, "snapshot_hz": m.Movement.SnapshotHz,
	}
}

func vec(p Point) combat.Vec   { return combat.Vec{X: p[0], Z: p[1]} }
func point(v combat.Vec) Point { return Point{v.X, v.Z} }

// Spawn places a character at its saved position, or at the spawn point
// when that position is no longer walkable (e.g. the map changed). A
// character that left the world dead comes back dead, without dropping again.
func (w *World) Spawn(id int64, name string, at Point, dead bool, sink Sink) (Point, Area) {
	w.mu.Lock()
	defer w.mu.Unlock()
	a, ok := w.nav.Classify(at[0], at[1])
	if !ok {
		at = w.spawn
		a, _ = w.nav.Classify(at[0], at[1])
	}
	f := w.eng.NewPlayer(id, vec(at), combat.Vec{X: 0, Z: 1}, "armed")
	if dead {
		f.HP, f.Dead = 0, true
	}
	w.eng.Add(f)
	w.ents[id] = &entity{id: id, name: name, pos: at, area: a, sink: sink, f: f}
	return at, a
}

// Attach gives a lingering character a connection again.
func (w *World) Attach(id int64, sink Sink) (Point, Area, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return Point{}, Area{}, false
	}
	e.sink = sink
	return e.pos, e.area, true
}

// Detach stops a disconnected character where it stands; it stays in the
// world, can still be attacked, but acts on nothing until reattached. Only
// the current sink may detach, so a replaced (kicked) connection cannot
// stall the new one.
func (w *World) Detach(id int64, sink Sink) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := w.ents[id]; e != nil && e.sink == sink {
		e.sink, e.path = nil, nil
	}
}

// Despawn removes a character and returns its last position and whether it
// was dead.
func (w *World) Despawn(id int64) (Point, bool, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return Point{}, false, false
	}
	delete(w.ents, id)
	w.eng.Remove(id)
	return e.pos, e.f.Dead, true
}

// MoveTo plans a route to target and starts walking it. A character
// already inside a risk area has accepted it and may move freely there.
func (w *World) MoveTo(id int64, target Point, acceptRisk bool, now time.Time) ([]Point, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return nil, ErrNotInWorld
	}
	if !e.f.CanMove(w.eng.Now) {
		return nil, ErrBusy
	}
	recent := e.moveTimes[:0]
	for _, t := range e.moveTimes {
		if now.Sub(t) < time.Second {
			recent = append(recent, t)
		}
	}
	e.moveTimes = recent
	if len(recent) >= w.limit {
		return nil, ErrRateLimited
	}
	e.moveTimes = append(e.moveTimes, now)
	path, err := w.nav.FindPath(e.pos, target, acceptRisk || e.area.Zone == ZoneRisk)
	if err != nil {
		return nil, err
	}
	e.path = path[1:]
	return path, nil
}

func (w *World) Stop(id int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return ErrNotInWorld
	}
	e.path = nil
	return nil
}

// Cast starts a skill; starting one stops walking (moves are rooted).
func (w *World) Cast(id int64, slot string, dir Point, target int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return ErrNotInWorld
	}
	e.f.Pos = vec(e.pos)
	if err := w.eng.Cast(id, slot, vec(dir), target); err != nil {
		return err
	}
	e.path = nil
	return nil
}

// Swap switches between armed and unarmed (1 s, no attacks meanwhile).
func (w *World) Swap(id int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ents[id] == nil {
		return ErrNotInWorld
	}
	return w.eng.Swap(id)
}

// Respawn brings a dead character back at the nearest safe point with full
// resources. Death costs carried goods only, never permanent growth.
func (w *World) Respawn(id int64) (Point, Area, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return Point{}, Area{}, ErrNotInWorld
	}
	if !e.f.Dead {
		return Point{}, Area{}, ErrNotDead
	}
	best := w.spawn
	for _, p := range w.safe {
		if dist(e.pos, p) < dist(e.pos, best) {
			best = p
		}
	}
	w.eng.Remove(id)
	e.f = w.eng.NewPlayer(id, vec(best), combat.Vec{X: 0, Z: 1}, e.f.Preset)
	w.eng.Add(e.f)
	e.pos, e.path = best, nil
	e.area, _ = w.nav.Classify(best[0], best[1])
	return best, e.area, nil
}

// SetHP sets a living character's health (tests and future GM tools). A
// value of 0 or less kills on the next tick through the normal death path.
func (w *World) SetHP(id int64, hp int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil || e.f.Dead {
		return false
	}
	e.f.HP = min(hp, e.f.MaxHP)
	return true
}

// AddCorpse places a corpse created by OnDeath (or loaded at startup).
func (w *World) AddCorpse(id int64, at Point, owner string, expires time.Time, empty bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := &corpse{id: id, pos: at, owner: owner, expires: expires}
	if empty {
		c.emptyAt = w.Now()
	}
	w.corpses[id] = c
}

// CorpseEmptied starts the 60 s fade of a corpse whose goods are all taken.
func (w *World) CorpseEmptied(id int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if c := w.corpses[id]; c != nil && c.emptyAt.IsZero() {
		c.emptyAt = w.Now()
	}
}

// CanLoot checks that a living character stands within reach of a corpse.
func (w *World) CanLoot(character, corpseID int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, c := w.ents[character], w.corpses[corpseID]
	switch {
	case e == nil:
		return ErrNotInWorld
	case e.f.Dead:
		return ErrDead
	case c == nil || dist(e.pos, c.pos) > LootRange:
		return ErrTooFar
	}
	return nil
}

// Step advances the world by one fixed tick of dt seconds.
func (w *World) Step(dt float64) {
	w.mu.Lock()
	deaths, expired := w.step(dt)
	w.mu.Unlock()
	w.runHooks(deaths, expired)
}

type death struct {
	id   int64
	name string
	at   Point
}

func (w *World) step(dt float64) ([]death, []int64) {
	w.tick++
	now := w.eng.Now
	for _, e := range w.ents {
		if !e.f.CanMove(now) {
			e.path = nil
			continue
		}
		budget := w.speed * dt * float64(w.eng.SpeedCenti(e.f)) / 100
		for budget > 0 && len(e.path) > 0 {
			next := e.path[0]
			d := dist(e.pos, next)
			if d > 1e-9 {
				e.f.Facing = combat.Vec{X: (next[0] - e.pos[0]) / d, Z: (next[1] - e.pos[1]) / d}
			}
			if d <= budget {
				e.pos, e.path = next, e.path[1:]
				budget -= d
				continue
			}
			f := budget / d
			e.pos = Point{e.pos[0] + (next[0]-e.pos[0])*f, e.pos[1] + (next[1]-e.pos[1])*f}
			budget = 0
		}
		e.f.Pos = vec(e.pos)
	}
	w.eng.Step()
	var deaths []death
	for _, ev := range w.eng.DrainEvents() {
		if len(w.pending) < maxPendingEvents {
			w.pending = append(w.pending, ev)
		}
		if ev.Kind == "death" {
			if e := w.ents[ev.Actor]; e != nil {
				deaths = append(deaths, death{e.id, e.name, point(e.f.Pos)})
			}
		}
	}
	for _, e := range w.ents {
		e.pos = point(e.f.Pos) // dashes move fighters
		if a, ok := w.nav.Classify(e.pos[0], e.pos[1]); ok {
			e.area = a
		}
	}
	var expired []int64
	wall := w.Now()
	for id, c := range w.corpses {
		if !wall.Before(c.expires) {
			expired = append(expired, id)
			delete(w.corpses, id)
		} else if !c.emptyAt.IsZero() && wall.Sub(c.emptyAt) >= EmptyCorpseLinger {
			delete(w.corpses, id)
		}
	}
	return deaths, expired
}

func (w *World) runHooks(deaths []death, expired []int64) {
	for _, d := range deaths {
		if w.Hooks.OnDeath != nil {
			w.Hooks.OnDeath(d.id, d.name, d.at)
		}
	}
	for _, id := range expired {
		if w.Hooks.OnCorpseExpired != nil {
			w.Hooks.OnCorpseExpired(id)
		}
	}
}

// EntityView is one character as others see it.
type EntityView struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Facing  float64 `json:"facing"` // radians, atan2(z, x)
	Area    string  `json:"area"`
	Zone    string  `json:"zone"`
	Moving  bool    `json:"moving"`
	HP      int     `json:"hp"`
	MaxHP   int     `json:"max_hp"`
	MP      int     `json:"mp"`
	SP      int     `json:"sp"`
	Dead    bool    `json:"dead"`
	Stunned bool    `json:"stunned"`
	Action  string  `json:"action,omitempty"` // skill being performed
	Preset  string  `json:"preset"`
}

type CorpseView struct {
	ID        int64   `json:"id"`
	X         float64 `json:"x"`
	Z         float64 `json:"z"`
	Owner     string  `json:"owner"`
	ExpiresIn int     `json:"expires_in"` // seconds
	Empty     bool    `json:"empty"`
}

type Snapshot struct {
	Tick     uint64         `json:"tick"`
	Entities []EntityView   `json:"entities"`
	Corpses  []CorpseView   `json:"corpses"`
	Events   []combat.Event `json:"events"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (w *World) entityView(e *entity) EntityView {
	f := e.f
	v := EntityView{ID: e.id, Name: e.name, X: round2(e.pos[0]), Z: round2(e.pos[1]),
		Facing: round2(math.Atan2(f.Facing.Z, f.Facing.X)), Area: e.area.ID, Zone: e.area.Zone,
		Moving: len(e.path) > 0, HP: f.HP, MaxHP: f.MaxHP, MP: f.MP, SP: f.SP, Dead: f.Dead,
		Stunned: f.Stunned(w.eng.Now), Preset: f.Preset}
	if s := f.ActionSkill(); s != "" {
		v.Action = s
	}
	return v
}

// Get returns one character's current view.
func (w *World) Get(id int64) (EntityView, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := w.ents[id]; e != nil {
		return w.entityView(e), true
	}
	return EntityView{}, false
}

type delivery struct {
	sink Sink
	snap Snapshot
}

// snapshots builds, for each connected character, the characters, corpses
// and combat events within view radius (itself included).
func (w *World) snapshots() []delivery {
	var out []delivery
	wall := w.Now()
	for _, viewer := range w.ents {
		if viewer.sink == nil {
			continue
		}
		s := Snapshot{Tick: w.tick, Entities: []EntityView{}, Corpses: []CorpseView{}, Events: []combat.Event{}}
		seen := map[int64]bool{}
		for _, e := range w.ents {
			if dist(viewer.pos, e.pos) <= w.view {
				s.Entities = append(s.Entities, w.entityView(e))
				seen[e.id] = true
			}
		}
		for _, c := range w.corpses {
			if dist(viewer.pos, c.pos) <= w.view {
				s.Corpses = append(s.Corpses, CorpseView{ID: c.id, X: round2(c.pos[0]), Z: round2(c.pos[1]), Owner: c.owner,
					ExpiresIn: int(math.Ceil(c.expires.Sub(wall).Seconds())), Empty: !c.emptyAt.IsZero()})
			}
		}
		for _, ev := range w.pending {
			if seen[ev.Actor] || seen[ev.Target] {
				s.Events = append(s.Events, ev)
			}
		}
		sort.Slice(s.Entities, func(i, j int) bool { return s.Entities[i].ID < s.Entities[j].ID })
		sort.Slice(s.Corpses, func(i, j int) bool { return s.Corpses[i].ID < s.Corpses[j].ID })
		out = append(out, delivery{viewer.sink, s})
	}
	w.pending = nil
	return out
}

// Positions returns every character's position and death state, for saving
// at shutdown.
func (w *World) Positions() map[int64]Saved {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[int64]Saved, len(w.ents))
	for id, e := range w.ents {
		out[id] = Saved{Pos: e.pos, Dead: e.f.Dead}
	}
	return out
}

type Saved struct {
	Pos  Point
	Dead bool
}

// Run ticks at the fixed rate until ctx ends. A late tick still advances one
// fixed step, so lag slows the world down and never speeds it up.
func (w *World) Run(ctx context.Context) {
	t := time.NewTicker(w.tickD)
	defer t.Stop()
	dt := w.tickD.Seconds()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		w.mu.Lock()
		deaths, expired := w.step(dt)
		var out []delivery
		if w.tick%uint64(w.every) == 0 {
			out = w.snapshots()
		}
		w.mu.Unlock()
		for _, d := range out {
			d.sink.Push("snapshot", d.snap)
		}
		w.runHooks(deaths, expired)
	}
}
