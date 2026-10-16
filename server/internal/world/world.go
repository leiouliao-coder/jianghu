// Package world is the authoritative world simulation. One loop owns every
// character's position; the block a character is in follows from its
// position, so it is always in exactly one block or road. Clients only send
// destinations: the server plans the path and moves at the fixed walk
// speed, so a client cannot choose its own speed or position.
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

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

var (
	ErrNotInWorld  = errors.New("world: character not in world")
	ErrRateLimited = errors.New("world: too many move requests")
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
}

type World struct {
	nav   *Nav
	speed float64
	view  float64
	tickD time.Duration
	every int // ticks per snapshot
	limit int // move requests per second
	spawn Point

	mu   sync.Mutex
	ents map[int64]*entity
	tick uint64
}

func New(d *gamedata.Data) *World {
	mv := d.Map.Movement
	return &World{
		nav:   NewNav(d),
		speed: mv.WalkSpeed,
		view:  mv.ViewRadius,
		tickD: time.Second / time.Duration(mv.TickHz),
		every: mv.TickHz / mv.SnapshotHz,
		limit: mv.MaxMoveRequestsPerSecond,
		spawn: Point{d.Map.Spawn.X, d.Map.Spawn.Z},
		ents:  map[int64]*entity{},
	}
}

func (w *World) Nav() *Nav { return w.nav }

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

// Spawn places a character at its saved position, or at the spawn point
// when that position is no longer walkable (e.g. the map changed).
func (w *World) Spawn(id int64, name string, at Point, sink Sink) (Point, Area) {
	w.mu.Lock()
	defer w.mu.Unlock()
	a, ok := w.nav.Classify(at[0], at[1])
	if !ok {
		at = w.spawn
		a, _ = w.nav.Classify(at[0], at[1])
	}
	w.ents[id] = &entity{id: id, name: name, pos: at, area: a, sink: sink}
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
// world but acts on nothing until reattached. Only the current sink may
// detach, so a replaced (kicked) connection cannot stall the new one.
func (w *World) Detach(id int64, sink Sink) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := w.ents[id]; e != nil && e.sink == sink {
		e.sink, e.path = nil, nil
	}
}

// Despawn removes a character and returns its last position.
func (w *World) Despawn(id int64) (Point, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.ents[id]
	if e == nil {
		return Point{}, false
	}
	delete(w.ents, id)
	return e.pos, true
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

// Step advances every walking character by speed*dt along its path.
func (w *World) Step(dt float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.step(dt)
}

func (w *World) step(dt float64) {
	w.tick++
	for _, e := range w.ents {
		budget := w.speed * dt
		for budget > 0 && len(e.path) > 0 {
			next := e.path[0]
			d := dist(e.pos, next)
			if d <= budget {
				e.pos, e.path = next, e.path[1:]
				budget -= d
				continue
			}
			f := budget / d
			e.pos = Point{e.pos[0] + (next[0]-e.pos[0])*f, e.pos[1] + (next[1]-e.pos[1])*f}
			budget = 0
		}
		if a, ok := w.nav.Classify(e.pos[0], e.pos[1]); ok {
			e.area = a
		}
	}
}

// EntityView is one character as others see it.
type EntityView struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Z      float64 `json:"z"`
	Area   string  `json:"area"`
	Zone   string  `json:"zone"`
	Moving bool    `json:"moving"`
}

type Snapshot struct {
	Tick     uint64       `json:"tick"`
	Entities []EntityView `json:"entities"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (e *entity) view() EntityView {
	return EntityView{ID: e.id, Name: e.name, X: round2(e.pos[0]), Z: round2(e.pos[1]),
		Area: e.area.ID, Zone: e.area.Zone, Moving: len(e.path) > 0}
}

// Get returns one character's current view.
func (w *World) Get(id int64) (EntityView, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := w.ents[id]; e != nil {
		return e.view(), true
	}
	return EntityView{}, false
}

type delivery struct {
	sink Sink
	snap Snapshot
}

// snapshots builds, for each connected character, the characters within
// view radius (itself included).
func (w *World) snapshots() []delivery {
	var out []delivery
	for _, viewer := range w.ents {
		if viewer.sink == nil {
			continue
		}
		s := Snapshot{Tick: w.tick}
		for _, e := range w.ents {
			if dist(viewer.pos, e.pos) <= w.view {
				s.Entities = append(s.Entities, e.view())
			}
		}
		sort.Slice(s.Entities, func(i, j int) bool { return s.Entities[i].ID < s.Entities[j].ID })
		out = append(out, delivery{viewer.sink, s})
	}
	return out
}

// Positions returns every character's position, for saving at shutdown.
func (w *World) Positions() map[int64]Point {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[int64]Point, len(w.ents))
	for id, e := range w.ents {
		out[id] = e.pos
	}
	return out
}

// Run ticks at the fixed rate until ctx ends. A late tick still advances one
// fixed step, so lag slows movement down and never speeds it up.
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
		w.step(dt)
		var out []delivery
		if w.tick%uint64(w.every) == 0 {
			out = w.snapshots()
		}
		w.mu.Unlock()
		for _, d := range out {
			d.sink.Push("snapshot", d.snap)
		}
	}
}
