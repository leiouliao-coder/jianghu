package crafting

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"testing"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

// Replays equipment_ledger_v41 (same flows as verify_equipment_ledger_v41.py)
// against the server's shared/data specs.
const ledgerPath = "../../../docs/design/v0.44/equipment_ledger_v41.json"

type ledgerItem struct {
	ID               string           `json:"id"`
	InitialInventory map[string]int64 `json:"initial_inventory"`
	Wear             []int64          `json:"wear"`
}

func load(t *testing.T) (*gamedata.Data, []ledgerItem) {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Items []ledgerItem }
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 2 {
		t.Fatalf("want 2 ledger items, got %d", len(f.Items))
	}
	return d, f.Items
}

// replayCache stands in for the M1 request table: a replayed request ID
// returns its first result without touching state.
type replayCache map[string]error

func (c replayCache) do(id string, op func() error) error {
	if err, ok := c[id]; ok {
		return err
	}
	err := op()
	if err == nil {
		c[id] = nil
	}
	return err
}

func TestEquipmentLifecycleMatchesLedger(t *testing.T) {
	d, items := load(t)
	for _, li := range items {
		spec := d.Equipment[li.ID]
		t.Run(spec.Name, func(t *testing.T) {
			stock := Stock(maps.Clone(li.InitialInventory))
			start := maps.Clone(stock)
			cache := replayCache{}
			var eq *Equipment
			if err := cache.do("make-1", func() (err error) { eq, err = Manufacture(stock, spec, spec.ID); return }); err != nil {
				t.Fatal(err)
			}
			for i, w := range li.Wear {
				if w > eq.Durability {
					t.Fatalf("ledger wear %d exceeds durability %d", w, eq.Durability)
				}
				if err := eq.Wear(w); err != nil {
					t.Fatal(err)
				}
				if i < 2 {
					if err := Repair(stock, eq, spec, spec.Profession, d.RepairMaxLoss); err != nil {
						t.Fatalf("repair %d: %v", i+1, err)
					}
				}
			}
			if eq.Durability != 0 || eq.MaxDurability != 96 || eq.Status != Disabled {
				t.Fatalf("after use: %+v, want D=0 M=96 disabled", eq)
			}
			if err := cache.do("salvage-1", func() error { return Salvage(stock, eq, spec, true) }); err != nil {
				t.Fatal(err)
			}
			if eq.Status != Destroyed {
				t.Fatal("original must leave circulation")
			}
			// Every raw material is used up exactly: recipe + two repairs.
			for k, have := range start {
				consumed := have - stock[k] + spec.Salvage[k]
				want := spec.Manufacture[k] + 2*spec.Repair[k]
				if consumed != want {
					t.Errorf("%s consumed %d, want %d", k, consumed, want)
				}
				if _, isScrap := spec.Salvage[k]; !isScrap && stock[k] != 0 {
					t.Errorf("%s left over %d, ledger expects 0", k, stock[k])
				}
			}
			// Scrap equivalent must stay below the main material it came from.
			scrap := spec.Salvage[spec.Scrap] * spec.ScrapEquivalent[0]
			if scrap >= spec.Manufacture[spec.MainMaterial]*spec.ScrapEquivalent[1] {
				t.Error("salvage returns as much main material as manufacture used")
			}
			after := maps.Clone(stock)
			if err := cache.do("salvage-duplicate", func() error { return Salvage(stock, eq, spec, true) }); !errors.Is(err, ErrGone) {
				t.Errorf("duplicate salvage err=%v", err)
			}
			if err := cache.do("salvage-1", func() error { return Salvage(stock, eq, spec, true) }); err != nil {
				t.Errorf("replayed salvage err=%v, want cached success", err)
			}
			if !maps.Equal(stock, after) || eq.Status != Destroyed {
				t.Error("duplicate or replayed salvage changed state")
			}
		})
	}
}

func TestRejectedOperationsChangeNothing(t *testing.T) {
	d, items := load(t)
	for _, li := range items {
		spec := d.Equipment[li.ID]
		fresh := func(t *testing.T) (Stock, *Equipment) {
			stock := Stock(maps.Clone(li.InitialInventory))
			eq, err := Manufacture(stock, spec, spec.ID)
			if err != nil {
				t.Fatal(err)
			}
			return stock, eq
		}
		cases := map[string]struct {
			run  func(t *testing.T) (Stock, *Equipment, func() error)
			want error
		}{
			"材料不足": {func(t *testing.T) (Stock, *Equipment, func() error) {
				stock := Stock(maps.Clone(li.InitialInventory))
				stock[spec.MainMaterial] = 0
				return stock, nil, func() error { _, err := Manufacture(stock, spec, spec.ID); return err }
			}, ErrMaterials},
			"错误专业": {func(t *testing.T) (Stock, *Equipment, func() error) {
				stock, eq := fresh(t)
				return stock, eq, func() error { return Repair(stock, eq, spec, "错误专业", d.RepairMaxLoss) }
			}, ErrProfession},
			"最大耐久为2": {func(t *testing.T) (Stock, *Equipment, func() error) {
				stock, eq := fresh(t)
				eq.Durability, eq.MaxDurability, eq.Status = 0, 2, Disabled
				return stock, eq, func() error { return Repair(stock, eq, spec, spec.Profession, d.RepairMaxLoss) }
			}, ErrMaxDurability},
			"未确认回收": {func(t *testing.T) (Stock, *Equipment, func() error) {
				stock, eq := fresh(t)
				return stock, eq, func() error { return Salvage(stock, eq, spec, false) }
			}, ErrNotConfirmed},
			"修理材料不足": {func(t *testing.T) (Stock, *Equipment, func() error) {
				stock, eq := fresh(t)
				clear(stock)
				return stock, eq, func() error { return Repair(stock, eq, spec, spec.Profession, d.RepairMaxLoss) }
			}, ErrMaterials},
		}
		for name, tc := range cases {
			t.Run(spec.Name+"/"+name, func(t *testing.T) {
				stock, eq, op := tc.run(t)
				before := maps.Clone(stock)
				var eqBefore Equipment
				if eq != nil {
					eqBefore = *eq
				}
				if err := op(); !errors.Is(err, tc.want) {
					t.Fatalf("err=%v want %v", err, tc.want)
				}
				if !maps.Equal(stock, before) || (eq != nil && *eq != eqBefore) {
					t.Error("rejected operation changed state")
				}
			})
		}
	}
}
