// Package ledger holds item-container rules shared by every transfer
// (baseline §1.4, §3.5): stack limits, slot packing and carry weight are
// prechecked so a rejected pickup, trade, withdrawal or load moves nothing.
package ledger

import (
	"errors"
	"fmt"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/growth"
)

var (
	ErrSlots  = errors.New("ledger: not enough free slots")
	ErrWeight = errors.New("ledger: carry weight exceeded")
	ErrItem   = errors.New("ledger: unknown item or bad quantity")
)

// Stack is one slot's content. Stacks merge only when Item and State match
// (quality, durability and other key state).
type Stack struct {
	Item  string
	State string
	Qty   int64
}

type Rules struct{ d *gamedata.Data }

func NewRules(d *gamedata.Data) Rules { return Rules{d} }

// WeightCenti is the total weight of stacks in 1/100 units.
func (r Rules) WeightCenti(stacks []Stack) (int64, error) {
	var w int64
	for _, s := range stacks {
		it, ok := r.d.Items[s.Item]
		if !ok || s.Qty <= 0 {
			return 0, fmt.Errorf("%w: %q x%d", ErrItem, s.Item, s.Qty)
		}
		w += it.WeightCenti * s.Qty
	}
	return w, nil
}

// CarryLimitCenti is 80 + effective strength, in the same 1/100 units.
func CarryLimitCenti(effectiveStrength growth.Centi) int64 {
	return 80*100 + int64(effectiveStrength)
}

// SlotsAfter packs incoming into held: top up matching stacks first, then
// open new slots. Held stacks are taken as already valid.
func (r Rules) SlotsAfter(held, incoming []Stack) (int, error) {
	room := map[[2]string]int64{}
	slots := len(held)
	for _, s := range held {
		limit := r.d.StackLimits[r.d.Items[s.Item].Class]
		room[[2]string{s.Item, s.State}] += max(0, limit-s.Qty)
	}
	for _, s := range incoming {
		it, ok := r.d.Items[s.Item]
		if !ok || s.Qty <= 0 {
			return 0, fmt.Errorf("%w: %q x%d", ErrItem, s.Item, s.Qty)
		}
		k := [2]string{s.Item, s.State}
		limit := r.d.StackLimits[it.Class]
		fill := min(room[k], s.Qty)
		room[k] -= fill
		rest := s.Qty - fill
		n := (rest + limit - 1) / limit
		slots += int(n)
		room[k] += n*limit - rest
	}
	return slots, nil
}

// CanReceive prechecks a transfer into a carried container. A character
// already over the limit may not add anything until they drop or store items.
func (r Rules) CanReceive(slotCap int, carryLimitCenti int64, held, incoming []Stack) error {
	slots, err := r.SlotsAfter(held, incoming)
	if err != nil {
		return err
	}
	if slots > slotCap {
		return fmt.Errorf("%w: need %d of %d", ErrSlots, slots, slotCap)
	}
	have, err := r.WeightCenti(held)
	if err != nil {
		return err
	}
	add, err := r.WeightCenti(incoming)
	if err != nil {
		return err
	}
	if have+add > carryLimitCenti {
		return fmt.Errorf("%w: %d+%d > %d (1/100 units)", ErrWeight, have, add, carryLimitCenti)
	}
	return nil
}
