package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
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

func TestDisconnectLingersUntilTheWorldReleases(t *testing.T) {
	p := NewPresence()
	var released []int64
	p.OnRelease = func(_, c int64) { released = append(released, c) }
	_ = p.Enter(1, 10, 100)
	p.Disconnect(1, 100)
	if c, s, ok := p.Current(1); !ok || c != 10 || s != Lingering {
		t.Fatalf("after disconnect: %d %s %v", c, s, ok)
	}
	var other *OtherCharacterPresent
	if err := p.Enter(1, 11, 101); !errors.As(err, &other) {
		t.Fatal("switching character while lingering must be refused")
	}
	if err := p.Leave(1, 100); !errors.Is(err, ErrNotPresent) {
		t.Fatal("the dead connection cannot log a lingering character out")
	}
	if p.ReleaseCharacter(99) {
		t.Fatal("released an unknown character")
	}
	if !p.ReleaseCharacter(10) || len(released) != 1 || released[0] != 10 {
		t.Fatalf("release: %v", released)
	}
	if _, _, ok := p.Current(1); ok {
		t.Fatal("lease still present after release")
	}
	if err := p.Enter(1, 11, 101); err != nil {
		t.Fatalf("any character may enter after release: %v", err)
	}
}

func TestReconnectTakesOverAndStaleDisconnectIsIgnored(t *testing.T) {
	p := NewPresence()
	_ = p.Enter(1, 10, 100)
	p.Disconnect(1, 100)
	if err := p.Enter(1, 10, 101); err != nil {
		t.Fatalf("reconnect to the same character: %v", err)
	}
	p.Disconnect(1, 100) // the old connection's late close
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
