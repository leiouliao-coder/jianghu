// Package ws is the real-time client gateway (protocol v1, see
// shared/protocol/v1.md). Each connection is read sequentially; replies and
// pushes go through one writer goroutine per connection.
package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/coder/websocket"

	"github.com/leiouliao-coder/jianghu/server/internal/session"
	"github.com/leiouliao-coder/jianghu/server/internal/store"
	"github.com/leiouliao-coder/jianghu/server/internal/world"
)

const ProtocolVersion = 1

// Close codes in the application range.
const (
	CloseKicked          websocket.StatusCode = 4001 // same account logged in elsewhere
	CloseTooManyFailures websocket.StatusCode = 4002
	CloseBadVersion      websocket.StatusCode = 4003
	CloseSlowConsumer    websocket.StatusCode = 4004
)

// Error codes sent in "error" payloads.
const (
	ErrBadRequest         = "bad_request"
	ErrUnsupportedVersion = "unsupported_version"
	ErrUnknownType        = "unknown_type"
	ErrNotAuthenticated   = "not_authenticated"
	ErrAlreadyAuth        = "already_authenticated"
	ErrBadCredentials     = "bad_credentials"
	ErrInvalidName        = "invalid_name"
	ErrInvalidPassword    = "invalid_password"
	ErrNameTaken          = "name_taken"
	ErrCharacterLimit     = "character_limit"
	ErrUnknownCharacter   = "unknown_character"
	ErrCharacterPresent   = "character_present"
	ErrNotInWorld         = "not_in_world"
	ErrUnreachable        = "unreachable"
	ErrRiskConfirmation   = "risk_confirmation_required"
	ErrRateLimited        = "rate_limited"
	ErrInternal           = "internal"
	ErrBusy               = "busy"
	ErrDead               = "dead"
	ErrNotDead            = "not_dead"
	ErrStunned            = "stunned"
	ErrCooldown           = "cooldown"
	ErrResources          = "insufficient_resources"
	ErrNoSkill            = "no_skill"
	ErrBadTarget          = "bad_target"
	ErrOutOfRange         = "out_of_range"
	ErrNoDirection        = "no_direction"
	ErrTooFar             = "too_far"
	ErrNotHere            = "not_here"
	ErrNoSpace            = "no_space"
)

// Envelope is every message in both directions.
type Envelope struct {
	V          int             `json:"v"`
	Type       string          `json:"type"`
	ReqID      string          `json:"req_id,omitempty"`
	Seq        uint64          `json:"seq,omitempty"`
	ServerTime int64           `json:"server_time,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type Config struct {
	SessionTTL       time.Duration
	IdleTimeout      time.Duration // close if the client sends nothing for this long
	MaxLoginFailures int
	Map              any               // world.ClientMap, sent with enter_world
	ItemNames        map[string]string // item ID -> display name
}

type Server struct {
	st       *store.Store
	presence *session.Presence
	world    *world.World
	cfg      Config

	mu        sync.Mutex
	byAccount map[int64]*conn
	all       map[*conn]struct{}
	nextID    atomic.Uint64
	closing   bool
	active    sync.WaitGroup // running ServeHTTP calls
}

// New wires the gateway to the store, the presence leases and the world.
// It takes over presence.OnRelease: a released character leaves the world
// and its position is saved.
// It also takes over the world's death and corpse hooks, which turn combat
// outcomes into asset transactions.
func New(st *store.Store, presence *session.Presence, w *world.World, cfg Config) *Server {
	s := &Server{st: st, presence: presence, world: w, cfg: cfg, byAccount: map[int64]*conn{}, all: map[*conn]struct{}{}}
	presence.OnRelease = s.onRelease
	w.Hooks.OnDeath = func(id int64, name string, at world.Point) { go s.dropOnDeath(id, name, at) }
	w.Hooks.OnCorpseExpired = func(id int64) { go s.expireCorpse(id) }
	// The world decides when a lingering or logging-out character has left.
	w.Hooks.OnLeft = func(id int64) { presence.ReleaseCharacter(id) }
	return s
}

func (s *Server) onRelease(_, character int64) {
	pos, dead, ok := s.world.Despawn(character)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.st.SaveState(ctx, character, pos[0], pos[1], dead); err != nil {
		log.Printf("ws: save state of character %d: %v", character, err)
	}
}

// SavePositions stores every character still in the world (at shutdown).
func (s *Server) SavePositions(ctx context.Context) {
	for id, p := range s.world.Positions() {
		if err := s.st.SaveState(ctx, id, p.Pos[0], p.Pos[1], p.Dead); err != nil {
			log.Printf("ws: save state of character %d: %v", id, err)
		}
	}
}

// LoadCorpses puts every live corpse back into the world at startup; ones
// that expired while the server was down expire on the first tick.
func (s *Server) LoadCorpses(ctx context.Context) error {
	corpses, err := s.st.LiveCorpses(ctx)
	if err != nil {
		return err
	}
	for _, c := range corpses {
		s.world.AddCorpse(c.ID, world.Point{c.X, c.Z}, c.OwnerName, c.ExpiresAt, c.Empty)
	}
	return nil
}

// dropOnDeath moves everything the character carried into a corpse at the
// death position. Retries reuse one request ID, so a retry after an
// uncertain failure can never drop twice.
func (s *Server) dropOnDeath(character int64, name string, at world.Point) {
	nonce := make([]byte, 8)
	_, _ = rand.Read(nonce)
	reqID := fmt.Sprintf("death:%d:%s", character, hex.EncodeToString(nonce))
	expires := time.Now().Add(world.CorpseTTL)
	for attempt := range 5 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		corpse, empty, err := s.drop(ctx, reqID, character, name, at, expires)
		cancel()
		if err == nil {
			s.world.AddCorpse(corpse, at, name, expires, empty)
			return
		}
		log.Printf("ws: death drop of character %d (attempt %d): %v", character, attempt+1, err)
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
}

func (s *Server) drop(ctx context.Context, reqID string, character int64, name string, at world.Point, expires time.Time) (int64, bool, error) {
	backpack, err := s.st.Backpack(ctx, character)
	if err != nil {
		return 0, false, err
	}
	res, err := s.st.Do(ctx, store.Request{ID: reqID, Kind: "death", Payload: character},
		func(ctx context.Context, tx *store.Tx) (any, error) {
			return tx.DropAll(ctx, backpack, name, at[0], at[1], expires)
		})
	if err != nil {
		return 0, false, err
	}
	if !res.OK {
		return 0, false, fmt.Errorf("drop rejected: %+v", res.Rejection)
	}
	var corpse int64
	if err := json.Unmarshal(res.Data, &corpse); err != nil {
		return 0, false, err
	}
	items, coins, err := s.st.CorpseContents(ctx, corpse)
	return corpse, err == nil && len(items) == 0 && coins == 0, err
}

func (s *Server) expireCorpse(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := s.st.Do(ctx, store.Request{ID: fmt.Sprintf("corpse-expire:%d", id), Kind: "corpse_expire", Payload: id},
		func(ctx context.Context, tx *store.Tx) (any, error) { return nil, tx.DestroyCorpse(ctx, id) })
	if err != nil || (!res.OK && res.Rejection.Code != store.CodeNotHere) {
		log.Printf("ws: expire corpse %d: %+v %v", id, res, err)
	}
}

type outMsg struct {
	data  []byte
	close websocket.StatusCode // nonzero: close with this code after data
	why   string
}

type conn struct {
	id        uint64
	nonce     string // random per connection; scopes pre-login request IDs
	ws        *websocket.Conn
	out       chan outMsg
	seq       atomic.Uint64
	sendMu    sync.Mutex
	once      sync.Once
	done      chan struct{}
	account   int64 // set under Server.mu
	failures  int   // read goroutine only
	character int64 // controlled character; read goroutine only
}

// Push implements world.Sink.
func (c *conn) Push(typ string, payload any) { c.send(typ, "", payload) }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// net/http clears its read/write deadlines when the connection is
	// hijacked, so long-lived sockets are not cut (TestIdleTimeout... checks).
	wsc, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	wsc.SetReadLimit(64 << 10)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce) // crypto/rand.Read never fails on supported platforms
	c := &conn{id: s.nextID.Add(1), nonce: hex.EncodeToString(nonce), ws: wsc, out: make(chan outMsg, 64), done: make(chan struct{})}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = wsc.Close(websocket.StatusGoingAway, "server shutting down")
		return
	}
	s.all[c] = struct{}{}
	s.active.Add(1)
	s.mu.Unlock()
	defer s.active.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.writer(ctx, c)
	s.readLoop(ctx, c)
	s.detach(c)
	c.closeWith(websocket.StatusNormalClosure, "")
	<-c.done
}

func (s *Server) readLoop(ctx context.Context, c *conn) {
	for {
		rctx, cancel := context.WithTimeout(ctx, s.cfg.IdleTimeout)
		typ, data, err := c.ws.Read(rctx)
		cancel()
		if err != nil {
			return
		}
		var env Envelope
		if typ != websocket.MessageText || json.Unmarshal(data, &env) != nil || env.Type == "" || !validReqID(env.ReqID) {
			c.sendError("", ErrBadRequest, "expected a JSON envelope with type and req_id (1-64 printable chars)", nil)
			continue
		}
		if env.V != ProtocolVersion {
			c.sendError(env.ReqID, ErrUnsupportedVersion, "server speaks protocol v1", map[string]int{"supported": ProtocolVersion})
			c.closeWith(CloseBadVersion, "unsupported protocol version")
			return
		}
		if !s.handle(ctx, c, env) {
			return
		}
	}
}

// writer is the only goroutine writing to the socket. It exits after a
// close message or a write error; c.out is never closed, so late senders
// simply see c.done.
func (s *Server) writer(ctx context.Context, c *conn) {
	defer close(c.done)
	for {
		var m outMsg
		select {
		case m = <-c.out:
		case <-ctx.Done():
			_ = c.ws.CloseNow()
			return
		}
		if m.data != nil {
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Write(wctx, websocket.MessageText, m.data)
			cancel()
			if err != nil {
				_ = c.ws.CloseNow()
				return
			}
		}
		if m.close != 0 {
			_ = c.ws.Close(m.close, m.why)
			return
		}
	}
}

func (c *conn) send(typ, reqID string, payload any) {
	// The lock keeps seq order equal to queue order across goroutines.
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	env := Envelope{V: ProtocolVersion, Type: typ, ReqID: reqID, Seq: c.seq.Add(1), ServerTime: time.Now().UnixMilli()}
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			log.Printf("ws: marshal %s: %v", typ, err)
			return
		}
		env.Payload = b
	}
	b, _ := json.Marshal(env) // only RawMessage and scalars: cannot fail
	select {
	case c.out <- outMsg{data: b}:
	case <-c.done:
	default:
		c.closeWith(CloseSlowConsumer, "client not reading")
	}
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

func (c *conn) sendError(reqID, code, msg string, detail any) {
	c.send("error", reqID, errorPayload{Code: code, Message: msg, Detail: detail})
}

// closeWith queues a close after already-queued messages. Safe to call more
// than once and from any goroutine.
func (c *conn) closeWith(code websocket.StatusCode, why string) {
	c.once.Do(func() {
		go func() {
			select {
			case c.out <- outMsg{close: code, why: why}:
			case <-c.done:
			}
		}()
	})
}

// attach binds an authenticated connection to its account and kicks any
// older connection of the same account.
func (s *Server) attach(c *conn, account int64) {
	s.mu.Lock()
	old := s.byAccount[account]
	s.byAccount[account] = c
	c.account = account
	s.mu.Unlock()
	if old != nil && old != c {
		old.send("kicked", "", map[string]string{"reason": "logged_in_elsewhere"})
		old.closeWith(CloseKicked, "logged in elsewhere")
	}
}

func (s *Server) detach(c *conn) {
	s.mu.Lock()
	account := c.account
	if account != 0 && s.byAccount[account] == c {
		delete(s.byAccount, account)
	}
	delete(s.all, c)
	s.mu.Unlock()
	if account != 0 {
		if c.character != 0 {
			s.world.Detach(c.character, c)
		}
		s.presence.Disconnect(account, c.id)
	}
}

func (s *Server) accountOf(c *conn) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return c.account
}

// Shutdown closes every connection with "going away" and waits for them to
// finish or ctx to end. Call it after http.Server.Shutdown, which ignores
// hijacked connections.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	conns := make([]*conn, 0, len(s.all))
	for c := range s.all {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		c.closeWith(websocket.StatusGoingAway, "server shutting down")
	}
	done := make(chan struct{})
	go func() { s.active.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func validReqID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
