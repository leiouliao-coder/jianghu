package combat

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"testing"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

// Replays the four authored scenarios of the move table (v0.35 §五). The
// casts and expected outcomes come from the appendix JSON; positions,
// facings and the fist's sidestep in A come from the scenario text.
const moveTable = "../../../docs/design/v0.44/剑拳内功轻功_招式样表_v0.35.md"

type scenario struct {
	ID     string  `json:"id"`
	End    float64 `json:"end"`
	Events []struct {
		T      float64 `json:"t"`
		Start  float64 `json:"start"`
		Actor  string  `json:"actor"`
		Skill  string  `json:"skill"`
		Result string  `json:"result"`
		Damage int     `json:"damage"`
	} `json:"events"`
	Refunds []struct {
		T     float64 `json:"t"`
		Actor string  `json:"actor"`
		MP    int     `json:"mp"`
		SP    int     `json:"sp"`
	} `json:"refunds"`
	Expected    map[string]struct{ HP, MP, SP int } `json:"expected"`
	RemainingCD map[string]float64                  `json:"remaining_cd"`
}

func loadData(t *testing.T) *gamedata.Data {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func loadScenarios(t *testing.T) []scenario {
	t.Helper()
	raw, err := os.ReadFile(moveTable)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("(?s)```json\n(.*?)\n```").FindSubmatch(raw)
	if m == nil {
		t.Fatal("no appendix JSON in the move table")
	}
	var f struct{ Scenarios []scenario }
	if err := json.Unmarshal(m[1], &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Scenarios) != 4 {
		t.Fatalf("want 4 scenarios, got %d", len(f.Scenarios))
	}
	return f.Scenarios
}

// setup per scenario text: sword faces +x from the origin; the fist stands
// on the x axis facing -x (x and z here are the text's x and y).
var setups = map[string]struct {
	fistX float64
	dash  Vec // sword_l destination offset, if any
}{
	"A": {fistX: 2},
	"B": {fistX: 2.8},
	"C": {fistX: 1.2},
	"D": {fistX: 6.3, dash: Vec{3, 0}},
}

func TestMoveTableScenarios(t *testing.T) {
	d := loadData(t)
	for _, sc := range loadScenarios(t) {
		t.Run(sc.ID, func(t *testing.T) {
			e := New(d)
			e.Regen = false // "场景时间内暂不自然恢复资源"
			tick := func(sec float64) Tick { return Ticks(sec, d.Combat.TickSeconds) }
			sword := e.NewPlayer(1, Vec{0, 0}, Vec{1, 0}, "armed")
			fist := e.NewPlayer(2, Vec{setups[sc.ID].fistX, 0}, Vec{-1, 0}, "unarmed")
			e.Add(sword)
			e.Add(fist)
			who := map[string]*Fighter{"sword": sword, "fist": fist}
			casts := map[Tick][]func(){}
			for _, ev := range sc.Events {
				f, sk := who[ev.Actor], d.Skills[ev.Skill]
				slot := e.skills[ev.Skill].Slot
				dir := f.Facing
				if sk.Shape == "dash" {
					dir = setups[sc.ID].dash
				}
				casts[tick(ev.Start)] = append(casts[tick(ev.Start)], func() {
					if err := e.Cast(f.ID, slot, dir, 0); err != nil {
						t.Fatalf("%s cast %s at %v: %v", ev.Actor, ev.Skill, ev.Start, err)
					}
				})
			}
			for now := Tick(0); now < tick(sc.End); now++ {
				for _, c := range casts[now] {
					c()
				}
				// Scenario A: the fist walks +y at 4 m/s until 0.35 s, then stops.
				if sc.ID == "A" && now < tick(0.35) {
					fist.Pos.Z += 4 * d.Combat.TickSeconds
				}
				e.Step()
			}
			for name, want := range sc.Expected {
				f := who[name]
				if f.HP != want.HP || f.MP != want.MP || f.SP != want.SP {
					t.Errorf("%s = %d/%d/%d, want %d/%d/%d", name, f.HP, f.MP, f.SP, want.HP, want.MP, want.SP)
				}
			}
			for skill, want := range sc.RemainingCD {
				f := who[map[bool]string{true: "sword", false: "fist"}[skill[:5] == "sword"]]
				got := float64(f.CooldownLeft(e.skills[skill].Slot, e.Now)) * d.Combat.TickSeconds
				if math.Abs(got-want) > 1e-9 {
					t.Errorf("%s cooldown left %.2f, want %.2f", skill, got, want)
				}
			}
			// Each authored outcome must appear in the event log at its time.
			for _, ev := range sc.Events {
				f := who[ev.Actor]
				switch ev.Result {
				case "落空":
					requireEvent(t, e, tick(ev.T), "miss", f.ID, ev.Skill, 0)
				case "格挡":
					requireEvent(t, e, tick(ev.T), "parry", sword.ID, ev.Skill, 0)
				case "命中", "命中并打断":
					requireEvent(t, e, tick(ev.T), "hit", f.ID, ev.Skill, ev.Damage)
				case "被打断":
					requireEvent(t, e, tick(ev.T), "interrupt", fist.ID, "", 0)
					for _, got := range e.Events() {
						if got.Kind == "hit" && got.Skill == ev.Skill {
							t.Errorf("interrupted %s still hit at tick %d", ev.Skill, got.Tick)
						}
					}
				}
			}
			for _, r := range sc.Refunds {
				requireEvent(t, e, tick(r.T), "refund", who[r.Actor].ID, "", r.MP+r.SP)
			}
		})
	}
}

func requireEvent(t *testing.T, e *Engine, at Tick, kind string, actor int64, skill string, value int) {
	t.Helper()
	for _, ev := range e.Events() {
		if ev.Tick == at && ev.Kind == kind && ev.Actor == actor && (skill == "" || ev.Skill == skill) && (value == 0 || ev.Value == value) {
			return
		}
	}
	t.Errorf("no %s event by %d (skill %q value %d) at tick %d; log: %+v", kind, actor, skill, value, at, e.Events())
}
