package ledger

import (
	"errors"
	"testing"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

func rules(t *testing.T) Rules {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	return NewRules(d)
}

func TestCarryLimit(t *testing.T) {
	// Baseline §3.5: effective strength 20 carries 100.
	if got := CarryLimitCenti(2000); got != 10000 {
		t.Fatalf("limit=%d want 10000", got)
	}
	if got := CarryLimitCenti(2010); got != 10010 {
		t.Fatalf("limit=%d want 10010", got)
	}
}

func TestWeights(t *testing.T) {
	r := rules(t)
	w, err := r.WeightCenti([]Stack{{Item: "测试铁剑"}, {Item: "TEST-SWORD-001", Qty: 1}})
	if !errors.Is(err, ErrItem) {
		t.Fatalf("unknown item accepted: %d %v", w, err)
	}
	w, err = r.WeightCenti([]Stack{
		{Item: "TEST-SWORD-001", Qty: 1}, {Item: "TEST-CLOTH-001", Qty: 1},
		{Item: "缝线", Qty: 4}, {Item: "碎布", Qty: 2}, {Item: "银钱", Qty: 250},
	})
	// 3 + 2 + 0.4 + 0.5 + 2.5
	if err != nil || w != 840 {
		t.Fatalf("weight=%d err=%v, want 840", w, err)
	}
}

func TestSlotPacking(t *testing.T) {
	r := rules(t)
	cases := []struct {
		name           string
		held, incoming []Stack
		want           int
	}{
		{"merge into partial stack", []Stack{{"铁锭", "", 60}}, []Stack{{"铁锭", "", 40}}, 1},
		{"overflow opens slot", []Stack{{"铁锭", "", 60}}, []Stack{{"铁锭", "", 41}}, 2},
		{"different state never merges", []Stack{{"铁锭", "", 60}}, []Stack{{"铁锭", "q2", 1}}, 2},
		{"equipment one per slot", nil, []Stack{{"TEST-SWORD-001", "d100", 3}}, 3},
		{"coin 10000 per slot", nil, []Stack{{"银钱", "", 10001}}, 2},
		{"incoming stacks merge with each other", nil, []Stack{{"布", "", 70}, {"布", "", 30}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.SlotsAfter(tc.held, tc.incoming)
			if err != nil || got != tc.want {
				t.Fatalf("slots=%d err=%v want %d", got, err, tc.want)
			}
		})
	}
}

func TestCanReceive(t *testing.T) {
	r := rules(t)
	limit := CarryLimitCenti(2000) // 100 weight
	held := []Stack{{"铁锭", "", 95}}
	if err := r.CanReceive(40, limit, held, []Stack{{"铁锭", "", 5}}); err != nil {
		t.Fatalf("exactly at limit must pass: %v", err)
	}
	if err := r.CanReceive(40, limit, held, []Stack{{"铁锭", "", 6}}); !errors.Is(err, ErrWeight) {
		t.Fatalf("over limit err=%v", err)
	}
	full := make([]Stack, 40)
	for i := range full {
		full[i] = Stack{"缝线", "", 1}
	}
	if err := r.CanReceive(40, limit, full, []Stack{{"布", "", 1}}); !errors.Is(err, ErrSlots) {
		t.Fatalf("full backpack err=%v", err)
	}
	if err := r.CanReceive(40, limit, full, []Stack{{"缝线", "", 99}}); err != nil {
		t.Fatalf("topping up an existing stack needs no slot: %v", err)
	}
	// Already overweight (e.g. strength dropped): nothing more may be added.
	over := []Stack{{"铁锭", "", 100}, {"铁锭", "", 5}}
	if err := r.CanReceive(40, limit, over, []Stack{{"缝线", "", 1}}); !errors.Is(err, ErrWeight) {
		t.Fatalf("overweight character added items: %v", err)
	}
}
