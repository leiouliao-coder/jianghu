// Package gamedata loads the versioned test data in shared/data and checks
// cross-file references. All values are test parameters, not final balance.
package gamedata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const SchemaVersion = 1

type ItemClass string

const (
	ClassMaterial   ItemClass = "material"
	ClassConsumable ItemClass = "consumable"
	ClassEquipment  ItemClass = "equipment"
	ClassCoin       ItemClass = "coin"
)

type Item struct {
	ID          string    `json:"id"`
	Class       ItemClass `json:"class"`
	WeightCenti int64     `json:"weight_centi"`
}

type Equipment struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Profession      string           `json:"profession"`
	Manufacture     map[string]int64 `json:"manufacture"`
	Repair          map[string]int64 `json:"repair"`
	Salvage         map[string]int64 `json:"salvage"`
	MaxDurability   int64            `json:"max_durability"`
	MainMaterial    string           `json:"main_material"`
	Scrap           string           `json:"scrap"`
	ScrapEquivalent [2]int64         `json:"scrap_equivalent"`
}

type Economy struct {
	MaterialPrices map[string]int64 `json:"npc_material_prices"`
	OrderRewards   map[string]int64 `json:"npc_order_rewards"`
	Pool           struct {
		Initial      int64 `json:"initial"`
		Cap          int64 `json:"cap"`
		HourlyRefill int64 `json:"hourly_refill"`
	} `json:"pool"`
	AccountLimit struct {
		Amount      int64 `json:"amount"`
		WindowHours int   `json:"window_hours"`
	} `json:"account_rolling_limit"`
	NodeSlotsPerAccount int `json:"node_slots_per_account"`
}

// World holds the slice of world.json the asset ledger needs.
type World struct {
	Characters struct {
		PerAccount int `json:"per_account"`
	} `json:"characters"`
	Inventory struct {
		BackpackSlots       int `json:"backpack_slots"`
		NodeSlotsPerAccount int `json:"node_slots_per_account"`
	} `json:"inventory"`
}

type Data struct {
	Items         map[string]Item
	StackLimits   map[ItemClass]int64
	Equipment     map[string]Equipment
	RepairMaxLoss int64
	Economy       Economy
	World         World
}

type header struct {
	SchemaVersion int    `json:"schema_version"`
	Status        string `json:"status"`
}

// Load reads shared/data from dir and rejects unknown schema versions,
// non-test status and dangling item references.
func Load(dir string) (*Data, error) {
	var items struct {
		header
		StackClasses map[ItemClass]int64 `json:"stack_classes"`
		Items        []Item              `json:"items"`
	}
	var recipes struct {
		header
		RepairMaxLoss int64       `json:"repair_max_loss"`
		Equipment     []Equipment `json:"equipment"`
	}
	var economy struct {
		header
		Economy
	}
	var world struct {
		header
		World
	}
	for name, dst := range map[string]any{"items.json": &items, "recipes.json": &recipes, "economy.json": &economy, "world.json": &world} {
		if err := readJSON(filepath.Join(dir, name), dst); err != nil {
			return nil, err
		}
	}
	for name, h := range map[string]header{"items.json": items.header, "recipes.json": recipes.header, "economy.json": economy.header, "world.json": world.header} {
		if h.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("%s: schema_version %d, want %d", name, h.SchemaVersion, SchemaVersion)
		}
		if h.Status != "test" {
			return nil, fmt.Errorf("%s: status %q, only test data exists", name, h.Status)
		}
	}
	d := &Data{Items: map[string]Item{}, StackLimits: items.StackClasses, Equipment: map[string]Equipment{},
		RepairMaxLoss: recipes.RepairMaxLoss, Economy: economy.Economy, World: world.World}
	var errs []error
	for _, it := range items.Items {
		if _, dup := d.Items[it.ID]; dup {
			errs = append(errs, fmt.Errorf("duplicate item %q", it.ID))
		}
		if d.StackLimits[it.Class] <= 0 {
			errs = append(errs, fmt.Errorf("item %q: unknown class %q", it.ID, it.Class))
		}
		if it.WeightCenti <= 0 {
			errs = append(errs, fmt.Errorf("item %q: weight must be positive", it.ID))
		}
		d.Items[it.ID] = it
	}
	knownQty := func(where string, m map[string]int64) {
		for id, q := range m {
			if _, ok := d.Items[id]; !ok {
				errs = append(errs, fmt.Errorf("%s: unknown item %q", where, id))
			}
			if q <= 0 {
				errs = append(errs, fmt.Errorf("%s: %q quantity must be positive", where, id))
			}
		}
	}
	for _, eq := range recipes.Equipment {
		if it, ok := d.Items[eq.ID]; !ok || it.Class != ClassEquipment {
			errs = append(errs, fmt.Errorf("equipment %q must be an equipment item", eq.ID))
		}
		knownQty(eq.ID+" manufacture", eq.Manufacture)
		knownQty(eq.ID+" repair", eq.Repair)
		knownQty(eq.ID+" salvage", eq.Salvage)
		if eq.MaxDurability <= recipes.RepairMaxLoss || eq.ScrapEquivalent[1] <= 0 {
			errs = append(errs, fmt.Errorf("equipment %q: invalid durability or scrap equivalent", eq.ID))
		}
		d.Equipment[eq.ID] = eq
	}
	knownQty("npc_material_prices", d.Economy.MaterialPrices)
	for name := range d.Economy.OrderRewards {
		if d.EquipmentByName(name) == nil {
			errs = append(errs, fmt.Errorf("npc_order_rewards: unknown equipment name %q", name))
		}
	}
	if p := d.Economy.Pool; p.Initial < 0 || p.Initial > p.Cap || p.HourlyRefill < 0 {
		errs = append(errs, errors.New("economy pool: need 0 <= initial <= cap and refill >= 0"))
	}
	if w := d.World; w.Characters.PerAccount <= 0 || w.Inventory.BackpackSlots <= 0 ||
		w.Inventory.NodeSlotsPerAccount != d.Economy.NodeSlotsPerAccount {
		errs = append(errs, errors.New("world: character/slot limits must be positive and node slots must match economy.json"))
	}
	if d.StackLimits[ClassCoin] <= 0 || len(d.ItemsOfClass(ClassCoin)) != 1 {
		errs = append(errs, errors.New("items: exactly one coin item with a positive stack limit is required"))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return d, nil
}

// ItemsOfClass lists item IDs of one class.
func (d *Data) ItemsOfClass(c ItemClass) []string {
	var ids []string
	for id, it := range d.Items {
		if it.Class == c {
			ids = append(ids, id)
		}
	}
	return ids
}

// EquipmentByName finds an equipment spec by display name, or nil.
func (d *Data) EquipmentByName(name string) *Equipment {
	for _, eq := range d.Equipment {
		if eq.Name == name {
			return &eq
		}
	}
	return nil
}

func readJSON(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
