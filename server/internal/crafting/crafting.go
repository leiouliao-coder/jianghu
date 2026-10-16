// Package crafting settles one equipment piece's lifecycle: manufacture,
// wear, repair and salvage (equipment_ledger_v41). Every operation validates
// everything before mutating, so a rejected call changes nothing.
// Request idempotency and storage belong to the transaction layer (M1).
package crafting

import (
	"errors"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

type Status string

const (
	Usable    Status = "usable"
	Disabled  Status = "disabled"  // zero durability: kept, but cannot be used
	Destroyed Status = "destroyed" // salvaged tombstone, out of circulation
)

var (
	ErrMaterials     = errors.New("crafting: insufficient materials")
	ErrProfession    = errors.New("crafting: profession does not match")
	ErrMaxDurability = errors.New("crafting: max durability too low to repair")
	ErrGone          = errors.New("crafting: equipment does not exist")
	ErrNotConfirmed  = errors.New("crafting: salvage not confirmed")
	ErrWear          = errors.New("crafting: wear must be positive")
)

// Stock is the material stack available to the action, by item ID.
type Stock map[string]int64

type Equipment struct {
	ID            string
	SpecID        string
	Durability    int64
	MaxDurability int64
	InitialMax    int64
	Status        Status
}

func (s Stock) covers(cost map[string]int64) bool {
	for k, v := range cost {
		if s[k] < v {
			return false
		}
	}
	return true
}

func (s Stock) apply(delta map[string]int64, sign int64) {
	for k, v := range delta {
		s[k] += sign * v
	}
}

// Manufacture consumes the recipe and creates the item with the given unique ID
// in the same step.
func Manufacture(stock Stock, spec gamedata.Equipment, id string) (*Equipment, error) {
	if !stock.covers(spec.Manufacture) {
		return nil, ErrMaterials
	}
	stock.apply(spec.Manufacture, -1)
	m := spec.MaxDurability
	return &Equipment{ID: id, SpecID: spec.ID, Durability: m, MaxDurability: m, InitialMax: m, Status: Usable}, nil
}

// Wear reduces durability; at zero the piece is disabled but still exists.
func (e *Equipment) Wear(amount int64) error {
	if e.Status == Destroyed {
		return ErrGone
	}
	if amount <= 0 {
		return ErrWear
	}
	e.Durability = max(0, e.Durability-amount)
	if e.Durability == 0 {
		e.Status = Disabled
	}
	return nil
}

// Repair restores durability to a max lowered by maxLoss each time.
func Repair(stock Stock, e *Equipment, spec gamedata.Equipment, profession string, maxLoss int64) error {
	switch {
	case e == nil || e.Status == Destroyed:
		return ErrGone
	case profession != spec.Profession:
		return ErrProfession
	case e.MaxDurability-maxLoss <= 0:
		return ErrMaxDurability
	case !stock.covers(spec.Repair):
		return ErrMaterials
	}
	stock.apply(spec.Repair, -1)
	e.MaxDurability -= maxLoss
	e.Durability = e.MaxDurability
	e.Status = Usable
	return nil
}

// Salvage destroys the original and yields only the limited scrap.
func Salvage(stock Stock, e *Equipment, spec gamedata.Equipment, confirmed bool) error {
	switch {
	case !confirmed:
		return ErrNotConfirmed
	case e == nil || e.Status == Destroyed:
		return ErrGone
	}
	e.Status = Destroyed
	stock.apply(spec.Salvage, 1)
	return nil
}
