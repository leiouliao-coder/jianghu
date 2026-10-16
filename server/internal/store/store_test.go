package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/store/storetest"
)

// testStore opens a migrated store on a fresh database (see storetest).
func testStore(t *testing.T) *Store {
	t.Helper()
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), storetest.DatabaseURL(t), d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

var ctx = context.Background()

// widenRaces makes every check-then-write pause, so concurrent requests in
// the calling test truly overlap.
func widenRaces(t *testing.T) {
	t.Helper()
	raceWindow = func(ctx context.Context, tx pgx.Tx) { _, _ = tx.Exec(ctx, `SELECT pg_sleep(0.02)`) }
	t.Cleanup(func() { raceWindow = func(context.Context, pgx.Tx) {} })
}

// run executes one system request and requires success.
func run[T any](t *testing.T, s *Store, id string, fn func(ctx context.Context, tx *Tx) (T, error)) T {
	t.Helper()
	var out T
	res, err := s.Do(ctx, Request{ID: id, Kind: "setup", Payload: id}, func(ctx context.Context, tx *Tx) (any, error) {
		v, err := fn(ctx, tx)
		out = v
		return v, err
	})
	if err != nil || !res.OK {
		t.Fatalf("%s: err=%v rejection=%+v", id, err, res.Rejection)
	}
	return out
}

type player struct{ account, character, backpack int64 }

func newPlayer(t *testing.T, s *Store, name string) player {
	t.Helper()
	return run(t, s, "setup-"+name, func(ctx context.Context, tx *Tx) (player, error) {
		a, err := tx.CreateAccount(ctx, name, "")
		if err != nil {
			return player{}, err
		}
		c, b, err := tx.CreateCharacter(ctx, a, name)
		return player{a, c, b}, err
	})
}

func newContainer(t *testing.T, s *Store, id, kind string, slots *int) int64 {
	t.Helper()
	return run(t, s, id, func(ctx context.Context, tx *Tx) (int64, error) { return tx.NewContainer(ctx, kind, "A", slots) })
}

func count(t *testing.T, s *Store, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := testStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, `SELECT count(*) FROM schema_migrations`); n != 2 {
		t.Fatalf("schema_migrations rows=%d", n)
	}
}

func TestReplayAppliesOnce(t *testing.T) {
	s := testStore(t)
	box := newContainer(t, s, "box", System, nil)
	grant := Request{ID: "grant-1", Kind: "grant", Payload: map[string]int64{"box": box, "coins": 100}}
	fn := func(ctx context.Context, tx *Tx) (any, error) { return box, tx.MintCoins(ctx, box, 100) }
	first, err := s.Do(ctx, grant, fn)
	if err != nil || !first.OK || first.Replayed {
		t.Fatalf("first: %+v %v", first, err)
	}
	for i := 0; i < 100; i++ {
		res, err := s.Do(ctx, grant, fn)
		if err != nil || !res.Replayed || string(res.Data) != string(first.Data) {
			t.Fatalf("replay %d: %+v %v", i, res, err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := s.Do(ctx, grant, fn); err != nil || !res.OK {
				t.Errorf("concurrent replay: %+v %v", res, err)
			}
		}()
	}
	wg.Wait()
	if c := count(t, s, `SELECT coins FROM containers WHERE id = $1`, box); c != 100 {
		t.Fatalf("coins=%d after replays, want 100", c)
	}
	if n := count(t, s, `SELECT count(*) FROM events WHERE request_id = 'grant-1'`); n != 1 {
		t.Fatalf("events=%d, want 1", n)
	}
}

func TestConcurrentDuplicateFirstAttempt(t *testing.T) {
	// 30 goroutines race the very first execution of one request ID.
	s := testStore(t)
	box := newContainer(t, s, "box", System, nil)
	req := Request{ID: "race", Kind: "grant", Payload: 7}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Do(ctx, req, func(ctx context.Context, tx *Tx) (any, error) { return nil, tx.MintCoins(ctx, box, 7) })
			if err != nil || !res.OK {
				t.Errorf("%+v %v", res, err)
			}
		}()
	}
	wg.Wait()
	if c := count(t, s, `SELECT coins FROM containers WHERE id = $1`, box); c != 7 {
		t.Fatalf("coins=%d, want 7", c)
	}
}

func TestRequestMismatch(t *testing.T) {
	s := testStore(t)
	p := newPlayer(t, s, "p")
	q := newPlayer(t, s, "q")
	noop := func(ctx context.Context, tx *Tx) (any, error) { return nil, nil }
	base := Request{ID: "r1", AccountID: p.account, Kind: "k", Payload: 1}
	if _, err := s.Do(ctx, base, noop); err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]Request{
		"payload": {ID: "r1", AccountID: p.account, Kind: "k", Payload: 2},
		"kind":    {ID: "r1", AccountID: p.account, Kind: "other", Payload: 1},
		"account": {ID: "r1", AccountID: q.account, Kind: "k", Payload: 1},
	} {
		if _, err := s.Do(ctx, req, noop); !errors.Is(err, ErrRequestMismatch) {
			t.Errorf("%s: err=%v, want ErrRequestMismatch", name, err)
		}
	}
	if _, err := s.Do(ctx, Request{Kind: "k"}, noop); !errors.Is(err, ErrBadRequest) {
		t.Errorf("empty id err=%v", err)
	}
}

func TestRejectionRollsBackAndIsReplayed(t *testing.T) {
	s := testStore(t)
	box := newContainer(t, s, "box", System, nil)
	req := Request{ID: "rej", Kind: "k", Payload: nil}
	calls := 0
	fn := func(ctx context.Context, tx *Tx) (any, error) {
		calls++
		if err := tx.MintCoins(ctx, box, 50); err != nil {
			return nil, err
		}
		return nil, reject(CodeInvalid, "rule refused after a write")
	}
	for i := 0; i < 3; i++ {
		res, err := s.Do(ctx, req, fn)
		if err != nil || res.OK || res.Rejection == nil || res.Rejection.Code != CodeInvalid {
			t.Fatalf("attempt %d: %+v %v", i, res, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fn ran %d times, want 1", calls)
	}
	if c := count(t, s, `SELECT coins FROM containers WHERE id = $1`, box); c != 0 {
		t.Fatalf("rejected write persisted: coins=%d", c)
	}
	if n := count(t, s, `SELECT count(*) FROM events WHERE request_id = 'rej'`); n != 0 {
		t.Fatalf("rejected request left %d events", n)
	}
}

func TestTransientErrorIsNotRecorded(t *testing.T) {
	s := testStore(t)
	box := newContainer(t, s, "box", System, nil)
	req := Request{ID: "flaky", Kind: "k", Payload: nil}
	boom := errors.New("connection blip")
	if _, err := s.Do(ctx, req, func(ctx context.Context, tx *Tx) (any, error) {
		_ = tx.MintCoins(ctx, box, 5)
		return nil, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	res, err := s.Do(ctx, req, func(ctx context.Context, tx *Tx) (any, error) { return nil, tx.MintCoins(ctx, box, 5) })
	if err != nil || !res.OK || res.Replayed {
		t.Fatalf("retry with same id: %+v %v", res, err)
	}
	if c := count(t, s, `SELECT coins FROM containers WHERE id = $1`, box); c != 5 {
		t.Fatalf("coins=%d, want 5", c)
	}
}

func TestConcurrentPickupExactlyOnce(t *testing.T) {
	s := testStore(t)
	widenRaces(t)
	corpse := newContainer(t, s, "corpse", Corpse, nil)
	sword := run(t, s, "drop", func(ctx context.Context, tx *Tx) (string, error) {
		return tx.Mint(ctx, corpse, "TEST-SWORD-001", "d100", 1)
	})
	const n = 50
	players := make([]player, n)
	for i := range players {
		players[i] = newPlayer(t, s, fmt.Sprintf("looter%02d", i))
	}
	results := make([]Result, n)
	var wg sync.WaitGroup
	for i, p := range players {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := Request{ID: fmt.Sprintf("pickup-%d", i), AccountID: p.account, Kind: "pickup", Payload: sword}
			res, err := s.Do(ctx, req, func(ctx context.Context, tx *Tx) (any, error) {
				return nil, tx.MoveItem(ctx, sword, corpse, p.backpack)
			})
			if err != nil {
				t.Error(err)
			}
			results[i] = res
		}()
	}
	wg.Wait()
	var winner int64
	ok := 0
	for i, r := range results {
		switch {
		case r.OK:
			ok++
			winner = players[i].backpack
		case r.Rejection == nil || r.Rejection.Code != CodeNotHere:
			t.Errorf("loser %d: %+v", i, r)
		}
	}
	if ok != 1 {
		t.Fatalf("%d pickups succeeded, want exactly 1", ok)
	}
	if loc := count(t, s, `SELECT container_id FROM items WHERE id = $1`, sword); loc != winner {
		t.Fatalf("sword in container %d, winner backpack %d", loc, winner)
	}
	if e := count(t, s, `SELECT count(*) FROM events WHERE kind = 'item_moved'`); e != 1 {
		t.Fatalf("item_moved events=%d", e)
	}
}

func TestConcurrentCoinsNeverNegative(t *testing.T) {
	s := testStore(t)
	widenRaces(t)
	purse := newContainer(t, s, "purse", Escrow, nil)
	run(t, s, "fund", func(ctx context.Context, tx *Tx) (any, error) { return nil, tx.MintCoins(ctx, purse, 1000) })
	const n = 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, short := 0, 0
	for i := 0; i < n; i++ {
		p := newPlayer(t, s, fmt.Sprintf("payee%02d", i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Do(ctx, Request{ID: fmt.Sprintf("pay-%d", i), AccountID: p.account, Kind: "pay", Payload: 30},
				func(ctx context.Context, tx *Tx) (any, error) { return nil, tx.MoveCoins(ctx, purse, p.backpack, 30) })
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				t.Error(err)
			case res.OK:
				ok++
			case res.Rejection.Code == CodeInsufficientCoins:
				short++
			default:
				t.Errorf("unexpected %+v", res)
			}
		}()
	}
	wg.Wait()
	if ok != 33 || short != 17 {
		t.Fatalf("ok=%d short=%d, want 33/17", ok, short)
	}
	if left := count(t, s, `SELECT coins FROM containers WHERE id = $1`, purse); left != 10 {
		t.Fatalf("purse=%d, want 10", left)
	}
	if total := count(t, s, `SELECT sum(coins) FROM containers`); total != 1000 {
		t.Fatalf("coins total=%d, want 1000 conserved", total)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE containers SET coins = -1 WHERE id = $1`, purse); err == nil {
		t.Fatal("database accepted a negative balance")
	}
}

func TestConcurrentLastSlot(t *testing.T) {
	s := testStore(t)
	widenRaces(t)
	one := 1
	shelf := newContainer(t, s, "shelf", NPCStock, &one)
	const n = 50
	type src struct {
		box  int64
		item string
	}
	srcs := make([]src, n)
	for i := range srcs {
		box := newContainer(t, s, fmt.Sprintf("src-%d", i), System, nil)
		item := run(t, s, fmt.Sprintf("mint-%d", i), func(ctx context.Context, tx *Tx) (string, error) {
			return tx.Mint(ctx, box, "铁锭", "", 1)
		})
		srcs[i] = src{box, item}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i, x := range srcs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Do(ctx, Request{ID: fmt.Sprintf("shelve-%d", i), Kind: "shelve", Payload: x.item},
				func(ctx context.Context, tx *Tx) (any, error) { return nil, tx.MoveItem(ctx, x.item, x.box, shelf) })
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if res.OK {
				ok++
			} else if res.Rejection.Code != CodeNoSlots {
				t.Errorf("unexpected %+v", res)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || count(t, s, `SELECT count(*) FROM items WHERE container_id = $1`, shelf) != 1 {
		t.Fatalf("ok=%d; last slot must be taken exactly once", ok)
	}
}

func TestCharacterLimitUnderConcurrency(t *testing.T) {
	s := testStore(t)
	acct := run(t, s, "acct", func(ctx context.Context, tx *Tx) (int64, error) { return tx.CreateAccount(ctx, "many", "") })
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Do(ctx, Request{ID: fmt.Sprintf("char-%d", i), AccountID: acct, Kind: "create_character", Payload: i},
				func(ctx context.Context, tx *Tx) (any, error) {
					_, _, err := tx.CreateCharacter(ctx, acct, fmt.Sprintf("hero%d", i))
					return nil, err
				})
			if err != nil || (!res.OK && res.Rejection.Code != CodeCharacterLimit) {
				t.Errorf("%+v %v", res, err)
			}
		}()
	}
	wg.Wait()
	if n := count(t, s, `SELECT count(*) FROM characters WHERE account_id = $1`, acct); n != 6 {
		t.Fatalf("characters=%d, want 6", n)
	}
	if n := count(t, s, `SELECT count(*) FROM containers WHERE kind = 'backpack'`); n != 6 {
		t.Fatalf("backpacks=%d, want 6", n)
	}
}

func TestBackpackWeightAndSlots(t *testing.T) {
	s := testStore(t)
	p := newPlayer(t, s, "carrier")
	do := func(id string, fn func(ctx context.Context, tx *Tx) error) Result {
		res, err := s.Do(ctx, Request{ID: id, Kind: "k", Payload: id}, func(ctx context.Context, tx *Tx) (any, error) { return nil, fn(ctx, tx) })
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	mint := func(proto string, qty int64) func(ctx context.Context, tx *Tx) error {
		return func(ctx context.Context, tx *Tx) error {
			_, err := tx.Mint(ctx, p.backpack, proto, "", qty)
			return err
		}
	}
	// Strength 20 carries exactly 100 weight (baseline §3.5).
	if r := do("iron", mint("铁锭", 100)); !r.OK {
		t.Fatalf("100 iron should fit: %+v", r)
	}
	if r := do("thread", mint("缝线", 1)); r.OK || r.Rejection.Code != CodeOverweight {
		t.Fatalf("over limit: %+v", r)
	}
	if r := do("coin", func(ctx context.Context, tx *Tx) error { return tx.MintCoins(ctx, p.backpack, 1) }); r.OK || r.Rejection.Code != CodeOverweight {
		t.Fatalf("coins have weight too: %+v", r)
	}
	if r := do("stack", mint("铁锭", 101)); r.OK || r.Rejection.Code != CodeInvalid {
		t.Fatalf("stack above limit: %+v", r)
	}
	q := newPlayer(t, s, "packer")
	for i := 0; i < 40; i++ {
		res, err := s.Do(ctx, Request{ID: fmt.Sprintf("fill-%d", i), Kind: "k", Payload: i}, func(ctx context.Context, tx *Tx) (any, error) {
			_, err := tx.Mint(ctx, q.backpack, "缝线", "", 1)
			return nil, err
		})
		if err != nil || !res.OK {
			t.Fatalf("fill %d: %+v %v", i, res, err)
		}
	}
	res, _ := s.Do(ctx, Request{ID: "fill-41", Kind: "k"}, func(ctx context.Context, tx *Tx) (any, error) {
		_, err := tx.Mint(ctx, q.backpack, "缝线", "", 1)
		return nil, err
	})
	if res.OK || res.Rejection.Code != CodeNoSlots {
		t.Fatalf("41st slot: %+v", res)
	}
}
