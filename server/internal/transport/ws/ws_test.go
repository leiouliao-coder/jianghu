package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/session"
	"github.com/leiouliao-coder/jianghu/server/internal/store"
	"github.com/leiouliao-coder/jianghu/server/internal/store/storetest"
	"github.com/leiouliao-coder/jianghu/server/internal/world"
)

// Integration tests over real WebSocket connections against PostgreSQL.

type harness struct {
	t   *testing.T
	url string
	gw  *Server
	w   *world.World
	st  *store.Store
}

// newHarness starts a gateway with a running world. linger, if set,
// shortens both departure delays (safe-zone disconnect and wild linger).
func newHarness(t *testing.T, cfg Config, linger ...time.Duration) *harness {
	t.Helper()
	d, err := gamedata.Load("../../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), storetest.DatabaseURL(t), d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = time.Hour
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 30 * time.Second
	}
	if cfg.MaxLoginFailures == 0 {
		cfg.MaxLoginFailures = 5
	}
	w := world.New(d)
	if len(linger) > 0 {
		w.SafeDisconnect, w.Linger = linger[0], linger[0]
	}
	cfg.Map = world.ClientMap(d)
	cfg.ItemNames = d.ItemNames()
	runCtx, stopWorld := context.WithCancel(context.Background())
	go w.Run(runCtx)
	t.Cleanup(stopWorld)
	gw := New(st, session.NewPresence(), w, cfg)
	srv := httptest.NewUnstartedServer(gw)
	// Same short deadlines as production; the gateway must lift them.
	srv.Config.ReadTimeout = 300 * time.Millisecond
	srv.Config.WriteTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = gw.Shutdown(ctx)
		srv.Close()
	})
	return &harness{t: t, url: "ws" + strings.TrimPrefix(srv.URL, "http"), gw: gw, w: w, st: st}
}

type client struct {
	t   *testing.T
	c   *websocket.Conn
	n   atomic.Int64
	seq uint64
}

func (h *harness) dial() *client {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, h.url, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { _ = c.CloseNow() })
	return &client{t: h.t, c: c}
}

func (c *client) write(v int, typ, reqID string, payload any) {
	c.t.Helper()
	env := map[string]any{"v": v, "type": typ, "req_id": reqID, "payload": payload}
	b, _ := json.Marshal(env)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.c.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
}

// read returns the next message, or the close error.
func (c *client) read() (Envelope, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := c.c.Read(ctx)
	if err != nil {
		return Envelope{}, err
	}
	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		c.t.Fatalf("bad server message %s", b)
	}
	if env.V != ProtocolVersion || env.Seq <= c.seq || env.ServerTime == 0 {
		c.t.Fatalf("envelope v/seq/time invalid: %+v (last seq %d)", env, c.seq)
	}
	c.seq = env.Seq
	return env, nil
}

// call sends a request and returns the reply to it.
func (c *client) call(typ string, payload any) Envelope {
	c.t.Helper()
	id := fmt.Sprintf("r%d", c.n.Add(1))
	c.write(1, typ, id, payload)
	return c.reply(id)
}

func (c *client) reply(id string) Envelope {
	c.t.Helper()
	for {
		env, err := c.read()
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", id, err)
		}
		if env.ReqID == id {
			return env
		}
	}
}

func (c *client) ok(typ string, payload any, out any) {
	c.t.Helper()
	env := c.call(typ, payload)
	if env.Type != typ+".ok" {
		c.t.Fatalf("%s: got %s %s", typ, env.Type, env.Payload)
	}
	if out != nil {
		if err := json.Unmarshal(env.Payload, out); err != nil {
			c.t.Fatal(err)
		}
	}
}

func (c *client) fails(typ string, payload any, code string) Envelope {
	c.t.Helper()
	env := c.call(typ, payload)
	var p errorPayload
	_ = json.Unmarshal(env.Payload, &p)
	if env.Type != "error" || p.Code != code {
		c.t.Fatalf("%s: want error %s, got %s %s", typ, code, env.Type, env.Payload)
	}
	return env
}

func (c *client) closedWith(code websocket.StatusCode) {
	c.t.Helper()
	for {
		_, err := c.read()
		if err == nil {
			continue
		}
		if got := websocket.CloseStatus(err); got != code {
			c.t.Fatalf("closed with %v (%v), want %v", got, err, code)
		}
		return
	}
}

type creds = map[string]string

func (c *client) login(name string) (account int64, token string) {
	c.t.Helper()
	var out struct {
		AccountID int64  `json:"account_id"`
		Token     string `json:"token"`
	}
	c.ok("login", creds{"name": name, "password": "password-" + name}, &out)
	return out.AccountID, out.Token
}

func (c *client) newCharacter(name string) int64 {
	c.t.Helper()
	var ch store.Character
	c.ok("char_create", map[string]string{"name": name}, &ch)
	return ch.ID
}

func registerAndLogin(h *harness, name string) (*client, int64) {
	c := h.dial()
	c.ok("register", creds{"name": name, "password": "password-" + name}, nil)
	acct, _ := c.login(name)
	return c, acct
}

func TestAccountAndCharacterFlow(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.dial()
	c.ok("ping", nil, nil)
	c.fails("char_list", nil, ErrNotAuthenticated)
	c.ok("register", creds{"name": "顾砚", "password": "password-顾砚"}, nil)
	c.fails("register", creds{"name": "顾砚", "password": "another-pass"}, ErrNameTaken)
	c.fails("register", creds{"name": "x", "password": "password-x"}, ErrInvalidName)
	c.fails("register", creds{"name": "short-pw", "password": "1234567"}, ErrInvalidPassword)
	acct, token := c.login("顾砚")
	if acct == 0 || len(token) < 40 {
		t.Fatalf("login gave %d %q", acct, token)
	}
	c.fails("login", creds{"name": "顾砚", "password": "password-顾砚"}, ErrAlreadyAuth)

	sword := c.newCharacter("剑客")
	fist := c.newCharacter("拳师")
	var list struct {
		Characters []store.Character `json:"characters"`
		Present    any               `json:"present"`
	}
	c.ok("char_list", nil, &list)
	if len(list.Characters) != 2 || list.Characters[0].ID != sword || list.Present != nil {
		t.Fatalf("char_list %+v", list)
	}
	c.fails("enter_world", map[string]int64{"character_id": 999999}, ErrUnknownCharacter)
	var entered struct {
		CharacterID int64          `json:"character_id"`
		Area        string         `json:"area"`
		Zone        string         `json:"zone"`
		X           float64        `json:"x"`
		Z           float64        `json:"z"`
		Map         map[string]any `json:"map"`
	}
	c.ok("enter_world", map[string]int64{"character_id": sword}, &entered)
	if entered.CharacterID != sword || entered.Area != "A" || entered.Zone != "service" || entered.Map["blocks"] == nil {
		t.Fatalf("enter_world %+v", entered)
	}
	c.fails("enter_world", map[string]int64{"character_id": fist}, ErrCharacterPresent)
	c.ok("leave_world", nil, nil)
	c.fails("leave_world", nil, ErrNotInWorld)
	c.ok("enter_world", map[string]int64{"character_id": fist}, nil)

	// Another account cannot enter someone else's character.
	other, _ := registerAndLogin(h, "陆平")
	other.fails("enter_world", map[string]int64{"character_id": sword}, ErrUnknownCharacter)
}

func TestCharacterCreateIsIdempotentAndLimited(t *testing.T) {
	h := newHarness(t, Config{})
	c, _ := registerAndLogin(h, "builder")
	c.write(1, "char_create", "same", map[string]string{"name": "hero0"})
	first := c.reply("same")
	c.write(1, "char_create", "same", map[string]string{"name": "hero0"})
	second := c.reply("same")
	if first.Type != "char_create.ok" || string(first.Payload) != string(second.Payload) {
		t.Fatalf("replay differs: %s / %s", first.Payload, second.Payload)
	}
	c.write(1, "char_create", "same", map[string]string{"name": "other"})
	if env := c.reply("same"); env.Type != "error" {
		t.Fatalf("reused req_id with new payload accepted: %s", env.Payload)
	}
	c.fails("char_create", map[string]string{"name": "hero0"}, ErrNameTaken)
	for i := 1; i < 6; i++ {
		c.newCharacter(fmt.Sprintf("hero%d", i))
	}
	c.fails("char_create", map[string]string{"name": "hero6"}, ErrCharacterLimit)
}

func TestBadCredentialsAreUniformAndLimited(t *testing.T) {
	h := newHarness(t, Config{MaxLoginFailures: 3})
	reg := h.dial()
	reg.ok("register", creds{"name": "known", "password": "password-known"}, nil)
	c := h.dial()
	a := c.fails("login", creds{"name": "unknown", "password": "whatever-pass"}, ErrBadCredentials)
	b := c.fails("login", creds{"name": "known", "password": "wrong-password"}, ErrBadCredentials)
	if string(a.Payload) != string(b.Payload) {
		t.Fatalf("unknown name and wrong password must look identical: %s vs %s", a.Payload, b.Payload)
	}
	c.fails("resume", map[string]string{"token": "forged"}, ErrBadCredentials)
	c.closedWith(CloseTooManyFailures)
}

func TestResumeWithToken(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.dial()
	c.ok("register", creds{"name": "resumer", "password": "password-resumer"}, nil)
	acct, token := c.login("resumer")
	r := h.dial()
	var out struct {
		AccountID int64 `json:"account_id"`
	}
	r.ok("resume", map[string]string{"token": token}, &out)
	if out.AccountID != acct {
		t.Fatalf("resume account %d want %d", out.AccountID, acct)
	}
	// The resumed connection replaced the first one.
	c.closedWith(CloseKicked)
}

func TestKickAndSingleCharacter(t *testing.T) {
	h := newHarness(t, Config{})
	a, _ := registerAndLogin(h, "twodevices")
	first := a.newCharacter("first")
	second := a.newCharacter("second")
	a.ok("enter_world", map[string]int64{"character_id": first}, nil)

	b := h.dial()
	b.login("twodevices")
	// Old device is told why, then closed.
	kicked := false
	for {
		env, err := a.read()
		if err != nil {
			if websocket.CloseStatus(err) != CloseKicked {
				t.Fatalf("old connection closed with %v", err)
			}
			break
		}
		if env.Type == "kicked" {
			kicked = strings.Contains(string(env.Payload), "logged_in_elsewhere")
		}
	}
	if !kicked {
		t.Fatal("old connection was closed without a kicked message")
	}
	// The character still holds the lease: only it may be (re)entered.
	var list struct {
		Present struct {
			CharacterID int64  `json:"character_id"`
			State       string `json:"state"`
		} `json:"present"`
	}
	b.ok("char_list", nil, &list)
	if list.Present.CharacterID != first {
		t.Fatalf("present %+v", list.Present)
	}
	env := b.fails("enter_world", map[string]int64{"character_id": second}, ErrCharacterPresent)
	if !strings.Contains(string(env.Payload), fmt.Sprint(first)) {
		t.Fatalf("error should name the present character: %s", env.Payload)
	}
	b.ok("enter_world", map[string]int64{"character_id": first}, nil)
	b.ok("leave_world", nil, nil)
	b.ok("enter_world", map[string]int64{"character_id": second}, nil)
}

func TestDisconnectLingerThenSwitch(t *testing.T) {
	// The linger must outlast the second login (argon2 is slow under -race
	// on CI runners), so it is seconds, not milliseconds.
	h := newHarness(t, Config{}, 3*time.Second)
	a, _ := registerAndLogin(h, "lingerer")
	first := a.newCharacter("stay")
	second := a.newCharacter("swap")
	a.ok("enter_world", map[string]int64{"character_id": first}, nil)
	_ = a.c.CloseNow() // abrupt network loss

	b := h.dial()
	b.login("lingerer")
	b.fails("enter_world", map[string]int64{"character_id": second}, ErrCharacterPresent)
	// Wait for the linger to end instead of sleeping a fixed time.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var list struct {
			Present *struct{} `json:"present"`
		}
		b.ok("char_list", nil, &list)
		if list.Present == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lingering character never left the world")
		}
		time.Sleep(100 * time.Millisecond)
	}
	b.ok("enter_world", map[string]int64{"character_id": second}, nil)
}

func TestProtocolErrors(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.dial()
	ctx := context.Background()
	_ = c.c.Write(ctx, websocket.MessageText, []byte("not json"))
	if env, _ := c.read(); env.Type != "error" || !strings.Contains(string(env.Payload), ErrBadRequest) {
		t.Fatalf("garbage: %+v", env)
	}
	c.write(1, "ping", strings.Repeat("x", 65), nil)
	if env, _ := c.read(); env.Type != "error" {
		t.Fatalf("long req_id accepted: %+v", env)
	}
	c.fails("teleport", nil, ErrUnknownType)
	c.fails("register", nil, ErrBadRequest)
	_ = c.c.Write(ctx, websocket.MessageBinary, []byte{1, 2})
	if env, _ := c.read(); env.Type != "error" {
		t.Fatalf("binary frame accepted: %+v", env)
	}
	c.write(2, "ping", "v2", nil)
	if env := c.reply("v2"); !strings.Contains(string(env.Payload), ErrUnsupportedVersion) {
		t.Fatalf("version: %s", env.Payload)
	}
	c.closedWith(CloseBadVersion)
}

func TestIdleTimeoutAndLongLivedConnection(t *testing.T) {
	h := newHarness(t, Config{IdleTimeout: 700 * time.Millisecond})
	c := h.dial()
	// Longer than the HTTP server's 300ms read/write deadlines: the gateway
	// must have cleared them, or this ping fails.
	for range 3 {
		time.Sleep(400 * time.Millisecond)
		c.ok("ping", nil, nil)
	}
	idle := h.dial()
	time.Sleep(1200 * time.Millisecond)
	if _, err := idle.read(); err == nil {
		t.Fatal("idle connection was not closed")
	}
}

func TestShutdownClosesConnections(t *testing.T) {
	h := newHarness(t, Config{})
	c := h.dial()
	c.ok("ping", nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The close handshake needs the client reading, as a real client always is.
	done := make(chan error, 1)
	go func() { done <- h.gw.Shutdown(ctx) }()
	c.closedWith(websocket.StatusGoingAway)
	if err := <-done; err != nil {
		t.Fatalf("shutdown did not drain: %v", err)
	}
	// Connections arriving after shutdown are turned away immediately.
	late := h.dial()
	late.closedWith(websocket.StatusGoingAway)
}

// snapshotUntil reads pushes until a snapshot satisfies ok, skipping replies.
func (c *client) snapshotUntil(ok func(world.Snapshot) bool) world.Snapshot {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		env, err := c.read()
		if err != nil {
			c.t.Fatalf("waiting for snapshot: %v", err)
		}
		if env.Type != "snapshot" {
			continue
		}
		var s world.Snapshot
		if err := json.Unmarshal(env.Payload, &s); err != nil {
			c.t.Fatal(err)
		}
		if ok(s) {
			return s
		}
	}
	c.t.Fatal("no matching snapshot within 5s")
	return world.Snapshot{}
}

func find(s world.Snapshot, id int64) (world.EntityView, bool) {
	for _, e := range s.Entities {
		if e.ID == id {
			return e, true
		}
	}
	return world.EntityView{}, false
}

func TestMoveAndSnapshots(t *testing.T) {
	h := newHarness(t, Config{})
	c, _ := registerAndLogin(h, "mover")
	id := c.newCharacter("行者")
	c.fails("move_to", map[string]float64{"x": 4, "z": 0}, ErrNotInWorld)
	c.ok("enter_world", map[string]int64{"character_id": id}, nil)
	c.fails("move_to", map[string]any{"x": 4}, ErrBadRequest)
	c.fails("move_to", map[string]float64{"x": -15, "z": -15}, ErrUnreachable) // inside a building
	c.fails("move_to", map[string]float64{"x": 200, "z": -130}, ErrRiskConfirmation)
	var moved struct {
		Path [][2]float64 `json:"path"`
	}
	start := time.Now()
	c.ok("move_to", map[string]float64{"x": 4, "z": 0}, &moved)
	if len(moved.Path) != 2 || moved.Path[1] != [2]float64{4, 0} {
		t.Fatalf("path %v", moved.Path)
	}
	// Client-supplied speed or position fields are ignored; the server walks 4 m/s.
	s := c.snapshotUntil(func(s world.Snapshot) bool {
		e, ok := find(s, id)
		return ok && !e.Moving && e.X == 4
	})
	if took := time.Since(start); took < 900*time.Millisecond {
		t.Fatalf("4 m covered in %v, faster than walking speed", took)
	}
	if e, _ := find(s, id); e.Area != "A" || e.Zone != "service" {
		t.Fatalf("area %+v", e)
	}
	c.ok("move_to", map[string]float64{"x": 20, "z": 0}, nil)
	c.ok("stop", nil, nil)
	c.snapshotUntil(func(s world.Snapshot) bool { e, ok := find(s, id); return ok && !e.Moving && e.X < 20 })
}

func TestPositionIsSavedOnLeave(t *testing.T) {
	h := newHarness(t, Config{})
	c, _ := registerAndLogin(h, "saver")
	id := c.newCharacter("归人")
	c.ok("enter_world", map[string]int64{"character_id": id}, nil)
	c.ok("move_to", map[string]float64{"x": 3, "z": 0}, nil)
	c.snapshotUntil(func(s world.Snapshot) bool { e, ok := find(s, id); return ok && !e.Moving && e.X == 3 })
	c.ok("leave_world", nil, nil)
	if _, ok := h.w.Get(id); ok {
		t.Fatal("character still in the world after leave_world")
	}
	var back struct{ X, Z float64 }
	c.ok("enter_world", map[string]int64{"character_id": id}, &back)
	if back.X != 3 || back.Z != 0 {
		t.Fatalf("re-entered at %v,%v, want 3,0", back.X, back.Z)
	}
}

func TestPlayersSeeEachOtherAndKickKeepsCharacter(t *testing.T) {
	h := newHarness(t, Config{}, 300*time.Millisecond)
	a, _ := registerAndLogin(h, "alpha")
	aid := a.newCharacter("甲士")
	a.ok("enter_world", map[string]int64{"character_id": aid}, nil)
	b, _ := registerAndLogin(h, "beta")
	bid := b.newCharacter("乙士")
	b.ok("enter_world", map[string]int64{"character_id": bid}, nil)
	b.snapshotUntil(func(s world.Snapshot) bool { _, ok := find(s, aid); return ok })

	// alpha logs in elsewhere: the character stays and the new device controls it.
	a2 := h.dial()
	a2.login("alpha")
	a.closedWith(CloseKicked)
	a2.ok("enter_world", map[string]int64{"character_id": aid}, nil)
	a2.ok("move_to", map[string]float64{"x": -3, "z": 0}, nil)
	a2.snapshotUntil(func(s world.Snapshot) bool { e, ok := find(s, aid); return ok && !e.Moving && e.X == -3 })

	// Network loss: the character lingers, then leaves the world.
	_ = a2.c.CloseNow()
	b.snapshotUntil(func(s world.Snapshot) bool { _, ok := find(s, aid); return !ok })
	if _, ok := h.w.Get(aid); ok {
		t.Fatal("lingering character not removed after the linger time")
	}
	info, err := h.st.CharacterInfo(context.Background(), aid)
	if err != nil || info.X != -3 {
		t.Fatalf("position after linger: %+v %v", info, err)
	}
}

// Before login, req_id is scoped to the connection: two clients that pick
// the same req_id must not collide (or squat on each other's IDs).
func TestRegisterRequestIDsAreScopedPerConnection(t *testing.T) {
	h := newHarness(t, Config{})
	for _, name := range []string{"first-user", "second-user"} {
		c := h.dial()
		c.write(1, "register", "same-id", creds{"name": name, "password": "password-" + name})
		if env := c.reply("same-id"); env.Type != "register.ok" {
			t.Fatalf("%s: %s %s", name, env.Type, env.Payload)
		}
	}
}

// placeAt sets where a character will enter the world.
func (h *harness) placeAt(id int64, x, z float64) {
	h.t.Helper()
	if err := h.st.SaveState(context.Background(), id, x, z, false); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) mintTo(character int64, reqID string, fn func(ctx context.Context, tx *store.Tx, backpack int64) error) {
	h.t.Helper()
	bp, err := h.st.Backpack(context.Background(), character)
	if err != nil {
		h.t.Fatal(err)
	}
	res, err := h.st.Do(context.Background(), store.Request{ID: reqID, Kind: "grant", Payload: reqID},
		func(ctx context.Context, tx *store.Tx) (any, error) { return nil, fn(ctx, tx, bp) })
	if err != nil || !res.OK {
		h.t.Fatalf("grant %s: %+v %v", reqID, res, err)
	}
}

func TestKillDropLootRespawn(t *testing.T) {
	h := newHarness(t, Config{})
	a, _ := registerAndLogin(h, "killer")
	aid := a.newCharacter("刀客")
	b, _ := registerAndLogin(h, "victim")
	bid := b.newCharacter("行商")
	h.placeAt(aid, 200, -130) // both inside the F risk area
	h.placeAt(bid, 201.2, -130)
	var sword string
	h.mintTo(bid, "kit-sword", func(ctx context.Context, tx *store.Tx, bp int64) (err error) {
		sword, err = tx.Mint(ctx, bp, "TEST-SWORD-001", "d100", 1)
		return err
	})
	h.mintTo(bid, "kit-coins", func(ctx context.Context, tx *store.Tx, bp int64) error { return tx.MintCoins(ctx, bp, 120) })
	a.ok("enter_world", map[string]int64{"character_id": aid}, nil)
	b.ok("enter_world", map[string]int64{"character_id": bid}, nil)
	a.snapshotUntil(func(s world.Snapshot) bool { e, ok := find(s, bid); return ok && e.Zone == "risk" })

	if !h.w.SetHP(bid, 50) {
		t.Fatal("set hp")
	}
	a.ok("cast", map[string]any{"slot": "basic", "target": bid}, nil)
	var corpseID int64
	s := a.snapshotUntil(func(s world.Snapshot) bool {
		e, _ := find(s, bid)
		return e.Dead && len(s.Corpses) == 1
	})
	corpseID = s.Corpses[0].ID
	if c := s.Corpses[0]; c.Owner != "行商" || c.Empty || c.ExpiresIn < 1790 || c.ExpiresIn > 1800 {
		t.Fatalf("corpse %+v", c)
	}
	if items, coins, _ := h.st.CorpseContents(context.Background(), corpseID); len(items) != 1 || coins != 120 {
		t.Fatalf("corpse holds %v + %d coins, want the sword and 120", items, coins)
	}
	b.fails("cast", map[string]any{"slot": "basic", "target": aid}, ErrDead)
	b.fails("move_to", map[string]float64{"x": 200, "z": -120}, ErrBusy)

	var opened struct {
		Items []struct {
			store.CorpseItem
			Name string `json:"name"`
		} `json:"items"`
		Coins int64 `json:"coins"`
	}
	a.ok("corpse_open", map[string]int64{"corpse_id": corpseID}, &opened)
	if len(opened.Items) != 1 || opened.Items[0].ID != sword || opened.Items[0].Name != "测试铁剑" || opened.Coins != 120 {
		t.Fatalf("corpse_open %+v", opened)
	}
	a.ok("loot", map[string]any{"corpse_id": corpseID, "item_id": sword}, nil)
	a.fails("loot", map[string]any{"corpse_id": corpseID, "item_id": sword}, ErrNotHere)
	a.fails("loot", map[string]any{"corpse_id": corpseID}, ErrBadRequest)
	var took struct{ Coins int64 }
	a.ok("loot", map[string]any{"corpse_id": corpseID, "coins": true}, &took)
	if took.Coins != 120 {
		t.Fatalf("took %d coins", took.Coins)
	}
	a.snapshotUntil(func(s world.Snapshot) bool { return len(s.Corpses) == 1 && s.Corpses[0].Empty })
	items, coins, err := h.st.CorpseContents(context.Background(), corpseID)
	if err != nil || len(items) != 0 || coins != 0 {
		t.Fatalf("corpse after looting: %v %d %v", items, coins, err)
	}

	var back struct {
		Area string `json:"area"`
		Zone string `json:"zone"`
	}
	b.ok("respawn", nil, &back)
	if back.Area != "B" || back.Zone != "service" {
		t.Fatalf("respawned in %+v, want nearest safe town B", back)
	}
	b.fails("respawn", nil, ErrNotDead)
}

func TestNoPlayerHarmOutsideRiskAndFarLooters(t *testing.T) {
	h := newHarness(t, Config{})
	a, _ := registerAndLogin(h, "peaceful")
	aid := a.newCharacter("和尚")
	b, _ := registerAndLogin(h, "bystander")
	bid := b.newCharacter("路人")
	h.placeAt(aid, 100, 0) // on the A-B road (pve)
	h.placeAt(bid, 101.2, 0)
	a.ok("enter_world", map[string]int64{"character_id": aid}, nil)
	b.ok("enter_world", map[string]int64{"character_id": bid}, nil)
	a.snapshotUntil(func(s world.Snapshot) bool { _, ok := find(s, bid); return ok })
	a.fails("cast", map[string]any{"slot": "basic", "target": bid}, ErrBadTarget)
	a.fails("cast", map[string]any{"slot": "zz", "dir": [2]float64{1, 0}}, ErrNoSkill)
	a.ok("cast", map[string]any{"slot": "e", "dir": [2]float64{1, 0}}, nil) // area moves start but skip protected players
	time.Sleep(1500 * time.Millisecond)
	if v, _ := h.w.Get(bid); v.HP != v.MaxHP {
		t.Fatalf("pve-zone player took damage: %+v", v)
	}
	a.fails("loot", map[string]any{"corpse_id": 999, "coins": true}, ErrTooFar)
}

func TestDeadCharacterComesBackDeadWithoutSecondDrop(t *testing.T) {
	h := newHarness(t, Config{}, 300*time.Millisecond)
	a, _ := registerAndLogin(h, "ghost")
	aid := a.newCharacter("孤魂")
	h.placeAt(aid, 200, -130)
	a.ok("enter_world", map[string]int64{"character_id": aid}, nil)
	h.w.SetHP(aid, 0)
	a.snapshotUntil(func(s world.Snapshot) bool { e, _ := find(s, aid); return e.Dead && len(s.Corpses) == 1 })
	// Outside a safe zone a logout is delayed; wait until the character has
	// really left before coming back.
	var leave struct {
		Left    bool `json:"left"`
		Delayed bool `json:"delayed"`
	}
	a.ok("leave_world", nil, &leave)
	if leave.Left || !leave.Delayed {
		t.Fatalf("logout in a risk area must be delayed: %+v", leave)
	}
	for {
		env, err := a.read()
		if err != nil {
			t.Fatal(err)
		}
		if env.Type == "left_world" {
			break
		}
	}
	if _, ok := h.w.Get(aid); ok {
		t.Fatal("character still in the world after left_world")
	}
	var back struct {
		Dead bool `json:"dead"`
	}
	a.ok("enter_world", map[string]int64{"character_id": aid}, &back)
	if !back.Dead {
		t.Fatal("left dead, must come back dead")
	}
	time.Sleep(300 * time.Millisecond)
	live, err := h.st.LiveCorpses(context.Background())
	if err != nil || len(live) != 1 {
		t.Fatalf("one death must leave exactly one corpse: %v %v", live, err)
	}
	a.ok("respawn", nil, nil)
}
