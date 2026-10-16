// Package session holds the world-wide presence lease: at most one character
// per account is in the world (baseline §4.1, state-machines.md §1). The
// lease outlives the connection: after a disconnect the character lingers,
// and until the lease ends the account may only reconnect that character.
//
// The lease is in memory because the world it guards runs in this process;
// when zones move to separate processes it must move to a shared store.
package session

import (
	"errors"
	"fmt"
	"sync"
	"time"
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
	timer     *time.Timer
	gen       uint64 // guards stale timers
}

type Presence struct {
	mu     sync.Mutex
	leases map[int64]*lease // by account
	gen    uint64
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
		if l.timer != nil {
			l.timer.Stop()
			l.timer = nil
		}
		l.owner, l.state = owner, Present
		return nil
	}
	p.leases[account] = &lease{character: character, owner: owner, state: Present}
	return nil
}

// Leave ends the lease immediately (a normal logout where that is allowed).
func (p *Presence) Leave(account int64, owner uint64) error {
	p.mu.Lock()
	l := p.leases[account]
	if l == nil || l.owner != owner || l.state != Present {
		p.mu.Unlock()
		return ErrNotPresent
	}
	p.releaseLocked(account, l)
	p.mu.Unlock()
	p.notify(account, l.character)
	return nil
}

// Disconnect marks owner's character as lingering; the lease ends after
// linger unless the character is re-entered first. A stale owner (already
// replaced by a newer connection) is ignored.
func (p *Presence) Disconnect(account int64, owner uint64, linger time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.leases[account]
	if l == nil || l.owner != owner || l.state != Present {
		return
	}
	p.gen++
	gen := p.gen
	l.state, l.gen = Lingering, gen
	l.timer = time.AfterFunc(linger, func() {
		p.mu.Lock()
		cur := p.leases[account]
		if cur != l || cur.gen != gen || cur.state != Lingering {
			p.mu.Unlock()
			return
		}
		p.releaseLocked(account, cur)
		p.mu.Unlock()
		p.notify(account, cur.character)
	})
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

func (p *Presence) releaseLocked(account int64, l *lease) {
	if l.timer != nil {
		l.timer.Stop()
	}
	delete(p.leases, account)
}

func (p *Presence) notify(account, character int64) {
	if p.OnRelease != nil {
		p.OnRelease(account, character)
	}
}
