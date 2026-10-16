package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOneCharacterPerAccount(t *testing.T) {
	p := NewPresence()
	if err := p.Enter(1, 10, 100); err != nil {
		t.Fatal(err)
	}
	var other *OtherCharacterPresent
	if err := p.Enter(1, 11, 100); !errors.As(err, &other) || other.CharacterID != 10 {
		t.Fatalf("second character err=%v", err)
	}
	if err := p.Enter(2, 20, 200); err != nil {
		t.Fatal("other accounts are independent")
	}
	if err := p.Leave(1, 999); !errors.Is(err, ErrNotPresent) {
		t.Fatal("a non-owner must not end the lease")
	}
	if err := p.Leave(1, 100); err != nil {
		t.Fatal(err)
	}
	if err := p.Enter(1, 11, 101); err != nil {
		t.Fatalf("after leave any character may enter: %v", err)
	}
}

func TestDisconnectLingersThenReleases(t *testing.T) {
	p := NewPresence()
	released := make(chan int64, 1)
	p.OnRelease = func(_, c int64) { released <- c }
	_ = p.Enter(1, 10, 100)
	p.Disconnect(1, 100, 50*time.Millisecond)
	if c, s, ok := p.Current(1); !ok || c != 10 || s != Lingering {
		t.Fatalf("after disconnect: %d %s %v", c, s, ok)
	}
	var other *OtherCharacterPresent
	if err := p.Enter(1, 11, 101); !errors.As(err, &other) {
		t.Fatal("switching character while lingering must be refused")
	}
	if err := p.Leave(1, 100); !errors.Is(err, ErrNotPresent) {
		t.Fatal("a lingering character cannot be logged out instantly by the dead connection")
	}
	select {
	case c := <-released:
		if c != 10 {
			t.Fatalf("released %d", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lease never released")
	}
	if _, _, ok := p.Current(1); ok {
		t.Fatal("lease still present after linger")
	}
}

func TestReconnectCancelsLinger(t *testing.T) {
	p := NewPresence()
	var releases atomic.Int32
	p.OnRelease = func(_, _ int64) { releases.Add(1) }
	_ = p.Enter(1, 10, 100)
	p.Disconnect(1, 100, 30*time.Millisecond)
	if err := p.Enter(1, 10, 101); err != nil {
		t.Fatalf("reconnect to the same character: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if c, s, ok := p.Current(1); !ok || c != 10 || s != Present || releases.Load() != 0 {
		t.Fatalf("stale timer released a reconnected character: %d %s %v %d", c, s, ok, releases.Load())
	}
	// The old connection's late disconnect must not affect the new owner.
	p.Disconnect(1, 100, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, s, _ := p.Current(1); s != Present {
		t.Fatal("stale owner disconnected the new connection's character")
	}
}

func TestConcurrentEnterDifferentCharacters(t *testing.T) {
	p := NewPresence()
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if p.Enter(1, int64(i), uint64(i)) == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("%d characters entered, want 1", ok.Load())
	}
}
