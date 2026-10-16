// Package economy is the NPC limited-purchase order book (economy_v44):
// per-node budget pools with a refill cap, frozen order rewards, real demand
// that only reopens when goods are actually consumed, and a rolling per-account
// income limit. It is a serial domain model; persistence and request
// idempotency are the transaction layer's job (M1).
package economy

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var (
	ErrDuplicateOrder = errors.New("economy: duplicate order id")
	ErrNoDemand       = errors.New("economy: no open demand")
	ErrNoBudget       = errors.New("economy: node budget insufficient")
	ErrAccountLimit   = errors.New("economy: account rolling limit reached")
	ErrNoCapacity     = errors.New("economy: no node capacity for payout")
	ErrNotOpen        = errors.New("economy: order not open")
	ErrAlreadyPaid    = errors.New("economy: order already paid")
	ErrInvalidItem    = errors.New("economy: item does not satisfy order")
	ErrNotInStock     = errors.New("economy: goods not in NPC stock")
)

type Params struct {
	PoolInitial, PoolCap, HourlyRefill int64
	AccountLimit                       int64
	WindowHours                        int
	NodeSlots                          int
	Rewards                            map[string]int64 // by item name
}

type OrderState string

const (
	Open      OrderState = "open"
	Paid      OrderState = "paid"
	Cancelled OrderState = "cancelled"
)

type Order struct {
	ID, Account, Node, Item string
	Reward                  int64
	State                   OrderState
}

// Goods describes the physical item offered for delivery. Pristine means
// full durability and never repaired.
type Goods struct {
	ID, Owner, Node, Name string
	Pristine              bool
}

// Payout is coin the caller must credit to the account's pending balance at
// the order's node; it never appears at another node.
type Payout struct {
	Account, Node string
	Amount        int64
}

type income struct {
	hour   int
	amount int64
}

type slotKey struct{ account, node string }

type Book struct {
	p             Params
	pools         map[string]int64
	need          map[string]map[string]int
	stock         map[string]map[string]map[string]bool // node → item → goods IDs
	orders        map[string]*Order
	orderSeq      []string
	income        map[string][]income
	reservedSlots map[slotKey]int
	slots         map[slotKey]int
	hour          int // running hours: drive refill
	realHour      int // wall hours incl. maintenance: drive rolling window
	mint          int64
	npcPaid       int64
}

func NewBook(p Params, nodes ...string) *Book {
	b := &Book{p: p, pools: map[string]int64{}, need: map[string]map[string]int{},
		stock: map[string]map[string]map[string]bool{}, orders: map[string]*Order{},
		income: map[string][]income{}, reservedSlots: map[slotKey]int{}, slots: map[slotKey]int{}}
	for _, n := range nodes {
		b.pools[n] = p.PoolInitial
	}
	return b
}

// SetDemand opens q units of real NPC need for item at node.
func (b *Book) SetDemand(node, item string, q int) {
	if b.need[node] == nil {
		b.need[node], b.stock[node] = map[string]int{}, map[string]map[string]bool{}
	}
	b.need[node][item] = q
	b.stock[node][item] = map[string]bool{}
}

func (b *Book) Frozen(node string) int64 {
	var f int64
	for _, o := range b.orders {
		if o.Node == node && o.State == Open {
			f += o.Reward
		}
	}
	return f
}

// Quota is income paid inside the rolling window plus open commitments; open
// orders never age out of it.
func (b *Book) Quota(account string) int64 {
	var q int64
	for _, in := range b.income[account] {
		if b.realHour-b.p.WindowHours < in.hour && in.hour <= b.realHour {
			q += in.amount
		}
	}
	for _, o := range b.orders {
		if o.Account == account && o.State == Open {
			q += o.Reward
		}
	}
	return q
}

func (b *Book) Accept(orderID, account, node, item string) error {
	if _, dup := b.orders[orderID]; dup {
		return ErrDuplicateOrder
	}
	reward := b.p.Rewards[item]
	held := 0
	for _, o := range b.orders {
		if o.Node == node && o.Item == item && o.State == Open {
			held++
		}
	}
	k := slotKey{account, node}
	switch {
	case b.need[node][item] <= held:
		return ErrNoDemand
	case b.pools[node]-b.Frozen(node) < reward:
		return ErrNoBudget
	case b.Quota(account)+reward > b.p.AccountLimit:
		return ErrAccountLimit
	case b.slots[k]+b.reservedSlots[k] >= b.p.NodeSlots:
		return ErrNoCapacity
	}
	b.reservedSlots[k]++
	b.orders[orderID] = &Order{ID: orderID, Account: account, Node: node, Item: item, Reward: reward, State: Open}
	b.orderSeq = append(b.orderSeq, orderID)
	return nil
}

// Cancel only unfreezes; it neither mints nor refunds anything.
func (b *Book) Cancel(orderID string) error {
	o := b.orders[orderID]
	if o == nil || o.State != Open {
		return ErrNotOpen
	}
	o.State = Cancelled
	b.reservedSlots[slotKey{o.Account, o.Node}]--
	return nil
}

// Deliver moves the goods into NPC stock and pays the frozen reward at the
// order's node. Offline owners can be paid: acceptance needs no presence.
func (b *Book) Deliver(orderID string, g Goods) (Payout, error) {
	o := b.orders[orderID]
	switch {
	case o == nil:
		return Payout{}, ErrNotOpen
	case o.State == Paid:
		return Payout{}, ErrAlreadyPaid
	case o.State != Open:
		return Payout{}, ErrNotOpen
	case g.Owner != o.Account || g.Node != o.Node || g.Name != o.Item || !g.Pristine:
		return Payout{}, ErrInvalidItem
	}
	b.need[o.Node][o.Item]--
	b.stock[o.Node][o.Item][g.ID] = true
	b.pools[o.Node] -= o.Reward
	b.npcPaid += o.Reward
	o.State = Paid
	b.income[o.Account] = append(b.income[o.Account], income{b.realHour, o.Reward})
	k := slotKey{o.Account, o.Node}
	b.reservedSlots[k]--
	b.slots[k]++
	return Payout{o.Account, o.Node, o.Reward}, nil
}

// Consume removes actual NPC-held goods from circulation, reopening demand.
func (b *Book) Consume(node, item string, ids []string) error {
	held := b.stock[node][item]
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !held[id] {
			return ErrNotInStock
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return ErrNotInStock
	}
	for _, id := range ids {
		delete(held, id)
	}
	b.need[node][item] += len(ids)
	return nil
}

// Refill advances running time; each pool grows by at most the hourly
// refill and never past the cap (frozen rewards count toward the cap).
func (b *Book) Refill(hours int) {
	for range hours {
		b.hour++
		b.realHour++
		for n, v := range b.pools {
			d := min(b.p.HourlyRefill, b.p.PoolCap-v)
			b.pools[n] += d
			b.mint += d
		}
	}
}

// Maintenance advances wall time only: no catch-up mint for downtime.
func (b *Book) Maintenance(hours int) { b.realHour += hours }

// Check verifies the book's invariants.
func (b *Book) Check() error {
	var errs []error
	for n, v := range b.pools {
		if f := b.Frozen(n); f < 0 || f > v || v > b.p.PoolCap {
			errs = append(errs, fmt.Errorf("node %s: need 0 <= frozen %d <= pool %d <= cap", n, f, v))
		}
	}
	for _, o := range b.orders {
		if q := b.Quota(o.Account); q > b.p.AccountLimit {
			errs = append(errs, fmt.Errorf("account %s quota %d over limit", o.Account, q))
		}
	}
	return errors.Join(errs...)
}

func (b *Book) Pool(node string) int64 { return b.pools[node] }
func (b *Book) PoolsTotal() (t int64) {
	for _, v := range b.pools {
		t += v
	}
	return
}
func (b *Book) Mint() int64                      { return b.mint }
func (b *Book) NPCPaid() int64                   { return b.npcPaid }
func (b *Book) Hours() (running, wall int)       { return b.hour, b.realHour }
func (b *Book) Order(id string) *Order           { o := *b.orders[id]; return &o }
func (b *Book) OrderIDs() []string               { return append([]string(nil), b.orderSeq...) }
func (b *Book) Nodes() []string                  { return sortedKeys(b.pools) }
func (b *Book) Need(node string) map[string]int  { return maps.Clone(b.need[node]) }
func (b *Book) DemandNodes() []string            { return sortedKeys(b.need) }
func (b *Book) StockCount(node, item string) int { return len(b.stock[node][item]) }

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }
