package world

import (
	"math"
	"sort"

	"github.com/leiouliao-coder/jianghu/server/internal/combat"
	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

// Monsters use negative IDs so they never collide with character IDs.
// Their AI is deliberately simple for the teaching fights (combat.json,
// authored M4 test parameters): wait at the spawn, chase the nearest
// player inside aggro range, attack when in reach, give up and walk home
// (healing fully) when dragged beyond the leash, respawn after death. They
// never enter service zones and drop nothing until loot tables exist.
type monster struct {
	kind      gamedata.MonsterKind
	kindID    string
	spawn     Point
	target    int64
	repathAt  combat.Tick
	respawnAt combat.Tick // set while dead
	homing    bool
}

// SpawnMonsters places every spawn from combat.json.
func (w *World) SpawnMonsters(d *gamedata.Data) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, sp := range d.Combat.Spawns {
		id := int64(-(i + 1))
		kind := d.Combat.Monsters[sp.Monster]
		w.addMonster(id, sp.Monster, kind, Point{sp.X, sp.Z})
	}
}

func (w *World) addMonster(id int64, kindID string, kind gamedata.MonsterKind, at Point) {
	f := w.eng.NewMonster(id, kind, vec(at))
	w.eng.Add(f)
	a, _ := w.nav.Classify(at[0], at[1])
	w.ents[id] = &entity{id: id, name: kind.Name, pos: at, area: a, f: f,
		mob: &monster{kind: kind, kindID: kindID, spawn: at}}
}

// monsterWalkable keeps monsters out of service zones (and off obstacles).
func (w *World) monsterWalkable(p Point) bool {
	a, ok := w.nav.Classify(p[0], p[1])
	return ok && a.Zone != ZoneService
}

// thinkMonsters runs before movement each tick.
func (w *World) thinkMonsters() {
	now := w.eng.Now
	ids := make([]int64, 0)
	for id, e := range w.ents {
		if e.mob != nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	for _, id := range ids {
		e := w.ents[id]
		m := e.mob
		if e.f.Dead {
			if m.respawnAt == 0 {
				m.respawnAt = now + w.ticks(m.kind.RespawnSeconds)
				e.path, m.target = nil, 0
			} else if now >= m.respawnAt {
				w.eng.Remove(id)
				w.addMonster(id, m.kindID, m.kind, m.spawn)
			}
			continue
		}
		if m.homing {
			if dist(e.pos, m.spawn) < 0.5 {
				m.homing = false
				e.f.HP = e.f.MaxHP
			}
			continue
		}
		if dist(e.pos, m.spawn) > m.kind.LeashRange {
			// Dragged too far: give up, walk home, heal on arrival.
			m.target, m.homing = 0, true
			if path, err := w.nav.FindPath(e.pos, m.spawn, true); err == nil {
				e.path = path[1:]
			}
			continue
		}
		t := w.ents[m.target]
		if t == nil || t.f.Dead || t.mob != nil || !w.CanHarm(e.f, t.f) || dist(e.pos, t.pos) > m.kind.LeashRange {
			m.target, t = 0, nil
		}
		if t == nil {
			best := math.MaxFloat64
			for _, c := range w.ents {
				if c.mob != nil || c.f.Dead || !w.CanHarm(e.f, c.f) {
					continue
				}
				if d := dist(e.pos, c.pos); d <= m.kind.AggroRange && d < best {
					best, t = d, c
				}
			}
			if t == nil {
				e.path = nil
				continue
			}
			m.target = t.id
		}
		reach := m.kind.Attack.Range
		if dist(e.pos, t.pos) <= reach {
			e.path = nil
			if e.f.CanMove(now) && e.f.CooldownLeft("basic", now) == 0 {
				_ = w.eng.Cast(id, "basic", Point{}.vec(), t.id)
			}
			continue
		}
		if now >= m.repathAt {
			m.repathAt = now + w.ticks(0.5)
			goal := approach(e.pos, t.pos, reach*0.8)
			if path, err := w.nav.FindPath(e.pos, goal, true); err == nil {
				e.path = path[1:]
			}
		}
	}
}

// approach is the point short of target by `stop` meters on the way there.
func approach(from, to Point, stop float64) Point {
	d := dist(from, to)
	if d <= stop {
		return from
	}
	f := (d - stop) / d
	return Point{from[0] + (to[0]-from[0])*f, from[1] + (to[1]-from[1])*f}
}

func (p Point) vec() combat.Vec { return vec(p) }

func (w *World) ticks(sec float64) combat.Tick { return combat.Ticks(sec, w.tickD.Seconds()) }
