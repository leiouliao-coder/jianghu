// Package growth settles the five base attributes in whole 0.1 steps
// (baseline §3.3). Values are fixed-point hundredths so no float rounding
// can create or lose points.
package growth

import (
	"fmt"
	"strconv"
	"strings"
)

// Centi is a fixed-point value in 1/100 units: 20.1 is Centi(2010).
type Centi int64

const (
	Step     Centi = 10   // one settlement step, 0.1
	Floor    Centi = 1000 // attribute minimum, 10
	FullStep Centi = 100  // progress needed for one step, 1.0
)

func ParseCenti(s string) (Centi, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	if whole == "" || len(frac) > 2 {
		return 0, fmt.Errorf("%q is not a number with at most two decimals", s)
	}
	n, err := strconv.ParseInt(whole+(frac + "00")[:2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", s, err)
	}
	return Centi(n), nil
}

func (c Centi) String() string {
	s := fmt.Sprintf("%d.%02d", c/100, c%100)
	return strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".0")
}

type Attr int

const (
	Strength     Attr = iota // 臂力
	Constitution             // 根骨
	Agility                  // 身法
	Insight                  // 悟性
	Composure                // 定力
	NumAttrs
)

var Names = [NumAttrs]string{"臂力", "根骨", "身法", "悟性", "定力"}

type Mode int

const (
	Raise Mode = iota // 提高
	Lock              // 锁定
	Lower             // 减少
)

type Outcome string

const (
	Grew           Outcome = "grew"          // budget not full, target +0.1
	Transferred    Outcome = "transferred"   // donor −0.1, target +0.1
	NotRaising     Outcome = "not_raising"   // target mode is not Raise; progress kept
	NeedProgress   Outcome = "need_progress" // less than one full step of progress
	ItemCapReached Outcome = "item_cap"      // target at its own cap; no donor touched
	NoDonor        Outcome = "no_donor"      // total full and no donor can give a whole step
)

// State is one character's permanent attributes. Equipment bonuses are
// effective values computed elsewhere and never stored here.
type State struct {
	Values   [NumAttrs]Centi
	Modes    [NumAttrs]Mode
	ItemCaps [NumAttrs]Centi
	TotalCap Centi
	Progress Centi  // banked training progress, at most one full step
	Priority []Attr // donor order chosen by the player
}

func New() State {
	s := State{TotalCap: 30000}
	for i := range s.Values {
		s.Values[i], s.ItemCaps[i] = 2000, 10000
	}
	return s
}

func (s *State) Total() Centi {
	var t Centi
	for _, v := range s.Values {
		t += v
	}
	return t
}

// Practice credits progress from one valid training action and settles at
// most one whole step. A blocked step keeps at most one step of progress, so
// repeated blocked practice cannot hoard points.
func (s *State) Practice(target Attr, earned Centi) (Outcome, Attr) {
	s.Progress = min(FullStep, s.Progress+earned)
	switch {
	case s.Modes[target] != Raise:
		return NotRaising, -1
	case s.Progress < FullStep:
		return NeedProgress, -1
	case s.Values[target]+Step > s.ItemCaps[target]:
		return ItemCapReached, -1
	}
	if s.Total()+Step <= s.TotalCap {
		s.Values[target] += Step
		s.Progress -= FullStep
		return Grew, -1
	}
	for _, d := range s.Priority {
		// A donor must give a whole step without dropping below the floor; never split.
		if d != target && s.Modes[d] == Lower && s.Values[d]-Step >= Floor {
			s.Values[d] -= Step
			s.Values[target] += Step
			s.Progress -= FullStep
			return Transferred, d
		}
	}
	return NoDonor, -1
}

// RaiseItemCap and RaiseTotalCap only lift limits; they never grant points.
func (s *State) RaiseItemCap(a Attr, to Centi) { s.ItemCaps[a] = max(s.ItemCaps[a], to) }
func (s *State) RaiseTotalCap(to Centi)        { s.TotalCap = max(s.TotalCap, to) }
