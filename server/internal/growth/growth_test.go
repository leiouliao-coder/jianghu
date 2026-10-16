package growth

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// Replays the 18 accepted cases in docs/design/v0.44/growth_cases_v37.json,
// the same fixture verify_growth_v37.py checks.
const casesPath = "../../../docs/design/v0.44/growth_cases_v37.json"

type growthCase struct {
	ID          string            `json:"id"`
	Before      []string          `json:"before"`
	Target      string            `json:"target"`
	Modes       []string          `json:"modes"`
	Priority    []string          `json:"priority"`
	Caps        map[string]string `json:"caps"`
	TotalCap    string            `json:"totalcap"`
	Earned      json.Number       `json:"earned"`
	Progress    string            `json:"progress"`
	Event       string            `json:"event"`
	RaisedCap   string            `json:"raised_cap"`
	RaisedTotal string            `json:"raised_total"`
	SkillBefore string            `json:"skill_before"`
	SkillGain   string            `json:"skill_gain"`
	SkillOK     bool              `json:"skill_eligible"`
	Threshold   string            `json:"trait_threshold"`
	Expect      struct {
		After       []string `json:"after"`
		Progress    string   `json:"progress"`
		Skill       string   `json:"skill"`
		TraitActive *bool    `json:"trait_active"`
	} `json:"expect"`
}

func mustCenti(t *testing.T, s, fallback string) Centi {
	t.Helper()
	if s == "" {
		s = fallback
	}
	c, err := ParseCenti(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func attr(t *testing.T, name string) Attr {
	t.Helper()
	i := slices.Index(Names[:], name)
	if i < 0 {
		t.Fatalf("unknown attribute %q", name)
	}
	return Attr(i)
}

func TestAcceptedGrowthCases(t *testing.T) {
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Cases []growthCase }
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) != 18 {
		t.Fatalf("got %d cases, want 18", len(file.Cases))
	}
	modeOf := map[string]Mode{"提高": Raise, "锁定": Lock, "减少": Lower}
	for _, c := range file.Cases {
		t.Run(c.ID, func(t *testing.T) {
			s := New()
			for i, v := range c.Before {
				s.Values[i] = mustCenti(t, v, "")
			}
			for i, m := range c.Modes {
				s.Modes[i] = modeOf[m]
			}
			for name, v := range c.Caps {
				s.ItemCaps[attr(t, name)] = mustCenti(t, v, "")
			}
			s.TotalCap = mustCenti(t, c.TotalCap, "300")
			s.Progress = mustCenti(t, c.Progress, "0")
			for _, p := range c.Priority {
				s.Priority = append(s.Priority, attr(t, p))
			}
			// Life skills settle independently of attributes (baseline §3.4):
			// a locked attribute never blocks a valid skill gain.
			skill := mustCenti(t, c.SkillBefore, "0")
			before := s
			target := attr(t, c.Target)
			switch c.Event {
			case "item_cap":
				s.RaiseItemCap(target, mustCenti(t, c.RaisedCap, ""))
			case "item_total":
				s.RaiseTotalCap(mustCenti(t, c.RaisedTotal, ""))
			case "equipment", "offline", "toggle":
				// Temporary bonuses, offline time and mode buttons settle nothing.
			default:
				if c.SkillOK {
					skill += mustCenti(t, c.SkillGain, "0.1")
				}
				s.Practice(target, mustCenti(t, c.Earned.String(), "0"))
			}
			for i, want := range c.Expect.After {
				if got := s.Values[i]; got != mustCenti(t, want, "") {
					t.Errorf("%s = %v, want %s", Names[i], got, want)
				}
				if s.Values[i] > s.ItemCaps[i] {
					t.Errorf("%s above its cap", Names[i])
				}
			}
			if s.Progress != mustCenti(t, c.Expect.Progress, "") || skill != mustCenti(t, c.Expect.Skill, "") {
				t.Errorf("progress=%v skill=%v, want %s %s", s.Progress, skill, c.Expect.Progress, c.Expect.Skill)
			}
			if s.Total() > s.TotalCap {
				t.Errorf("total %v above cap %v", s.Total(), s.TotalCap)
			}
			if c.Event != "" && c.Event != "practice" && s.Values != before.Values {
				t.Error("cap/offline/toggle/equipment events must not change attributes")
			}
			if c.Expect.TraitActive != nil {
				if got := s.Values[Insight] >= mustCenti(t, c.Threshold, "50"); got != *c.Expect.TraitActive {
					t.Errorf("insight trait active=%v", got)
				}
			}
		})
	}
}

func TestParseCenti(t *testing.T) {
	for in, want := range map[string]Centi{"20": 2000, "20.1": 2010, "10.05": 1005, "0.6": 60, "8": 800} {
		if got, err := ParseCenti(in); err != nil || got != want {
			t.Errorf("ParseCenti(%q)=%v,%v want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"1.234", "x", ""} {
		if _, err := ParseCenti(bad); err == nil {
			t.Errorf("ParseCenti(%q) accepted", bad)
		}
	}
	if got := Centi(2010).String(); got != "20.1" {
		t.Errorf("String()=%q", got)
	}
}
