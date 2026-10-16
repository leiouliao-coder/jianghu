package world

import (
	"container/heap"
	"errors"
	"math"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

var (
	ErrUnreachable  = errors.New("world: target is not reachable")
	ErrRiskRequired = errors.New("world: route enters a risk area; confirmation required")
)

// Zone permission classes (world.json zone_rules).
const (
	ZoneService = "service"
	ZonePvE     = "pve"
	ZoneRisk    = "risk"
)

// Area is where a point lies: a block, a road between blocks, or a risk
// area. A point is in exactly one area or not walkable.
type Area struct {
	ID   string `json:"area"`
	Zone string `json:"zone"`
}

type Point [2]float64 // x, z in meters

// navCell values.
const (
	blocked byte = iota
	open
	risky
)

// Nav is the walkable map at 1 m resolution plus exact geometry for
// classification and line-of-sight checks.
type Nav struct {
	m     gamedata.Map
	zones map[string]string // block id -> zone
	x0    float64
	z0    float64
	w, h  int
	cells []byte
}

const cellSize = 1.0

func NewNav(d *gamedata.Data) *Nav {
	n := &Nav{m: d.Map, zones: map[string]string{}}
	for _, b := range d.World.Blocks {
		n.zones[b.ID] = b.Zone
	}
	b := d.Map.Bounds
	n.x0, n.z0 = b[0], b[1]
	n.w, n.h = int(math.Ceil((b[2]-b[0])/cellSize)), int(math.Ceil((b[3]-b[1])/cellSize))
	n.cells = make([]byte, n.w*n.h)
	for i := range n.cells {
		x, z := n.center(i)
		if a, ok := n.Classify(x, z); ok {
			n.cells[i] = open
			if a.Zone == ZoneRisk {
				n.cells[i] = risky
			}
		}
	}
	return n
}

// Classify finds the area of a point; obstacles and off-road ground are not walkable.
func (n *Nav) Classify(x, z float64) (Area, bool) {
	if !n.m.Bounds.Contains(x, z) {
		return Area{}, false
	}
	for _, o := range n.m.Obstacles {
		if o.Rect.Contains(x, z) {
			return Area{}, false
		}
	}
	for _, r := range n.m.RiskAreas {
		if r.Rect.Contains(x, z) {
			return Area{ID: r.ID, Zone: ZoneRisk}, true
		}
	}
	for _, b := range n.m.Blocks {
		if b.Rect.Contains(x, z) {
			return Area{ID: b.ID, Zone: n.zones[b.ID]}, true
		}
	}
	for _, r := range n.m.Roads {
		for i := 1; i < len(r.Points); i++ {
			if segDist(Point{x, z}, r.Points[i-1], r.Points[i]) <= r.Width/2 {
				return Area{ID: r.ID, Zone: r.Zone}, true
			}
		}
	}
	return Area{}, false
}

func (n *Nav) passable(x, z float64, allowRisk bool) bool {
	a, ok := n.Classify(x, z)
	return ok && (allowRisk || a.Zone != ZoneRisk)
}

// FindPath returns waypoints from start to target (both included). Without
// allowRisk the route never enters a risk area; ErrRiskRequired means a
// route exists only through one.
func (n *Nav) FindPath(start, target Point, allowRisk bool) ([]Point, error) {
	ta, ok := n.Classify(target[0], target[1])
	if !ok {
		return nil, ErrUnreachable
	}
	if ta.Zone == ZoneRisk && !allowRisk {
		return nil, ErrRiskRequired
	}
	cells := n.astar(n.cellOf(start), n.cellOf(target), allowRisk)
	if cells == nil {
		if !allowRisk && n.astar(n.cellOf(start), n.cellOf(target), true) != nil {
			return nil, ErrRiskRequired
		}
		return nil, ErrUnreachable
	}
	raw := make([]Point, 0, len(cells)+1)
	raw = append(raw, start)
	for _, c := range cells[1:max(1, len(cells)-1)] {
		x, z := n.center(c)
		raw = append(raw, Point{x, z})
	}
	raw = append(raw, target)
	return n.smooth(raw, allowRisk), nil
}

// smooth keeps only waypoints needed for clear straight segments.
func (n *Nav) smooth(raw []Point, allowRisk bool) []Point {
	out := []Point{raw[0]}
	i := 0
	for i < len(raw)-1 {
		j := len(raw) - 1
		for j > i+1 && !n.clear(raw[i], raw[j], allowRisk) {
			j--
		}
		out = append(out, raw[j])
		i = j
	}
	return out
}

// clear samples a segment every 0.25 m.
func (n *Nav) clear(a, b Point, allowRisk bool) bool {
	d := dist(a, b)
	steps := int(math.Ceil(d / 0.25))
	for s := 0; s <= steps; s++ {
		t := 1.0
		if steps > 0 {
			t = float64(s) / float64(steps)
		}
		if !n.passable(a[0]+(b[0]-a[0])*t, a[1]+(b[1]-a[1])*t, allowRisk) {
			return false
		}
	}
	return true
}

func (n *Nav) cellOf(p Point) int {
	cx := min(max(int((p[0]-n.x0)/cellSize), 0), n.w-1)
	cz := min(max(int((p[1]-n.z0)/cellSize), 0), n.h-1)
	return cz*n.w + cx
}

func (n *Nav) center(i int) (float64, float64) {
	return n.x0 + (float64(i%n.w)+0.5)*cellSize, n.z0 + (float64(i/n.w)+0.5)*cellSize
}

func (n *Nav) cellOK(i int, allowRisk bool) bool {
	return n.cells[i] == open || (allowRisk && n.cells[i] == risky)
}

// astar searches 8-connected cells; diagonals may not cut blocked corners.
// The start cell is always usable so a character on a cell edge can leave.
func (n *Nav) astar(from, to int, allowRisk bool) []int {
	if !n.cellOK(to, allowRisk) {
		return nil
	}
	tx, tz := to%n.w, to/n.w
	h := func(i int) float64 {
		dx, dz := math.Abs(float64(i%n.w-tx)), math.Abs(float64(i/n.w-tz))
		return dx + dz + (math.Sqrt2-2)*math.Min(dx, dz)
	}
	g := map[int]float64{from: 0}
	prev := map[int]int{}
	closed := map[int]bool{}
	frontier := &pq{{from, h(from)}}
	for frontier.Len() > 0 {
		cur := heap.Pop(frontier).(node).cell
		if cur == to {
			path := []int{cur}
			for cur != from {
				cur = prev[cur]
				path = append(path, cur)
			}
			for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
				path[l], path[r] = path[r], path[l]
			}
			return path
		}
		if closed[cur] {
			continue
		}
		closed[cur] = true
		cx, cz := cur%n.w, cur/n.w
		for dz := -1; dz <= 1; dz++ {
			for dx := -1; dx <= 1; dx++ {
				nx, nz := cx+dx, cz+dz
				if (dx == 0 && dz == 0) || nx < 0 || nz < 0 || nx >= n.w || nz >= n.h {
					continue
				}
				next := nz*n.w + nx
				if closed[next] || !n.cellOK(next, allowRisk) {
					continue
				}
				cost := 1.0
				if dx != 0 && dz != 0 {
					if !n.cellOK(cz*n.w+nx, allowRisk) || !n.cellOK(nz*n.w+cx, allowRisk) {
						continue
					}
					cost = math.Sqrt2
				}
				if ng := g[cur] + cost; !hasLower(g, next, ng) {
					g[next] = ng
					prev[next] = cur
					heap.Push(frontier, node{next, ng + h(next)})
				}
			}
		}
	}
	return nil
}

func hasLower(g map[int]float64, i int, v float64) bool {
	old, ok := g[i]
	return ok && old <= v
}

type node struct {
	cell int
	f    float64
}

type pq []node

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].f < q[j].f }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(node)) }
func (q *pq) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

func dist(a, b Point) float64 { return math.Hypot(b[0]-a[0], b[1]-a[1]) }

func segDist(p Point, a, b [2]float64) float64 {
	dx, dz := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dz*dz
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dz)/l2))
	}
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dz))
}
