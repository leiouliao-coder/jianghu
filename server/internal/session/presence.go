// Package session holds the world-wide presence lease: at most one character
// per account is in the world (baseline §4.1, state-machines.md §1). The
// lease outlives the connection: after a disconnect the character lingers,
// and until the lease ends the account may only reconnect that character.
//
// The lease has no timer of its own. How long a character lingers depends
// on combat and zone, which the world loop knows, so the world ends the
// lease with ReleaseCharacter when the character has actually left.
//
// The lease is in memory because the world it guards runs in this process;
// when zones move to separate processes it must move to a shared store.
package session

import (
	"errors"
	"fmt"
	"sync"
)

var ErrNotPresent = errors.New("session: no lease for this owner")

// OtherCharacterPresent rejects entering while another character of the
// same account still holds the lease.
type OtherCharacterPresent struct{ CharacterID int64 }

func (e *OtherCharacterPresent) Error() string {
	return fmt.Sprintf("session: character %d is still in the world", e.CharacterID)
}

type State string

const (
	Present   State = "present"
	Lingering State = "lingering" // disconnected, still in the world
)

type lease struct {
	character int64
	owner     uint64 // connection that currently controls the character
	state     State
}

type Presence struct {
	mu     sync.Mutex
	leases map[int64]*lease // by account
	// OnRelease, if set, is called after a lease ends (outside the lock).
	OnRelease func(account, character int64)
}

func NewPresence() *Presence { return &Presence{leases: map[int64]*lease{}} }

// Enter gives owner control of character. Re-entering the leased character
// (a reconnect, or a new device after a kick) takes it over; any other
// character is refused until the lease ends.
func (p *Presence) Enter(account, character int64, owner uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l := p.leases[account]; l != nil {
		if l.character != character {
			return &OtherCharacterPresent{CharacterID: l.character}
		}
		l.owner, l.state = owner, Present
		return nil
	}
	p.leases[account] = &lease{character: character, owner: owner, state: Present}
	return nil
}

// Leave ends the lease immediately (a logout the world has allowed).
func (p *Presence) Leave(account int64, owner uint64) error {
	p.mu.Lock()
	l := p.leases[account]
	if l == nil || l.owner != owner || l.state != Present {
		p.mu.Unlock()
		return ErrNotPresent
	}
	delete(p.leases, account)
	p.mu.Unlock()
	p.notify(account, l.character)
	return nil
}

// Disconnect marks owner's character as lingering. A stale owner (already
// replaced by a newer connection) is ignored.
func (p *Presence) Disconnect(account int64, owner uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l := p.leases[account]; l != nil && l.owner == owner && l.state == Present {
		l.state = Lingering
	}
}

// ReleaseCharacter ends the lease holding character, whatever its state;
// the world calls it when the character has actually left. It reports
// whether a lease ended.
func (p *Presence) ReleaseCharacter(character int64) bool {
	p.mu.Lock()
	var account int64
	found := false
	for a, l := range p.leases {
		if l.character == character {
			account, found = a, true
			delete(p.leases, a)
			break
		}
	}
	p.mu.Unlock()
	if found {
		p.notify(account, character)
	}
	return found
}

// Current reports the account's lease.
func (p *Presence) Current(account int64) (character int64, state State, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l := p.leases[account]; l != nil {
		return l.character, l.state, true
	}
	return 0, "", false
}

func (p *Presence) notify(account, character int64) {
	if p.OnRelease != nil {
		p.OnRelease(account, character)
	}
}
