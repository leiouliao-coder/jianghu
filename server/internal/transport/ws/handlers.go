package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/leiouliao-coder/jianghu/server/internal/auth"
	"github.com/leiouliao-coder/jianghu/server/internal/session"
	"github.com/leiouliao-coder/jianghu/server/internal/store"
	"github.com/leiouliao-coder/jianghu/server/internal/world"
)

// handle processes one request; false closes the connection.
func (s *Server) handle(ctx context.Context, c *conn, env Envelope) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	switch env.Type {
	case "ping":
		c.send("ping.ok", env.ReqID, nil)
	case "register":
		s.register(ctx, c, env)
	case "login":
		return s.login(ctx, c, env)
	case "resume":
		return s.resume(ctx, c, env)
	case "char_list", "char_create", "enter_world", "leave_world", "move_to", "stop",
		"cast", "swap", "respawn", "corpse_open", "loot":
		account := s.accountOf(c)
		if account == 0 {
			c.sendError(env.ReqID, ErrNotAuthenticated, "log in or resume first", nil)
			return true
		}
		switch env.Type {
		case "char_list":
			s.charList(ctx, c, env, account)
		case "char_create":
			s.charCreate(ctx, c, env, account)
		case "enter_world":
			s.enterWorld(ctx, c, env, account)
		case "leave_world":
			s.leaveWorld(c, env, account)
		case "move_to":
			s.moveTo(c, env)
		case "stop":
			s.stop(c, env)
		default:
			if c.character == 0 {
				c.sendError(env.ReqID, ErrNotInWorld, "enter the world first", nil)
				return true
			}
			s.combatRequest(ctx, c, env, account)
		}
	default:
		c.sendError(env.ReqID, ErrUnknownType, "unknown message type "+env.Type, nil)
	}
	return true
}

func decode(c *conn, env Envelope, dst any) bool {
	if len(env.Payload) == 0 || string(env.Payload) == "null" || json.Unmarshal(env.Payload, dst) != nil {
		c.sendError(env.ReqID, ErrBadRequest, "payload does not match "+env.Type, nil)
		return false
	}
	return true
}

func (s *Server) internal(c *conn, env Envelope, err error) {
	log.Printf("ws: %s req=%s: %v", env.Type, env.ReqID, err)
	c.sendError(env.ReqID, ErrInternal, "internal error; retry with the same req_id", nil)
}

type credentials struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (s *Server) register(ctx context.Context, c *conn, env Envelope) {
	var p credentials
	if !decode(c, env, &p) {
		return
	}
	if auth.ValidName(p.Name) != nil {
		c.sendError(env.ReqID, ErrInvalidName, "name must be 2-32 letters, digits, _ or -", nil)
		return
	}
	hash, err := auth.HashPassword(p.Password)
	if err != nil {
		c.sendError(env.ReqID, ErrInvalidPassword, "password must be 8-128 bytes", nil)
		return
	}
	// Before login there is no account to scope req_id by, so it is scoped to
	// this connection: clients cannot collide with or squat on each other's
	// IDs. A retry after reconnecting gets name_taken and should log in.
	// The password is not part of the request fingerprint: only the name is.
	res, err := s.st.Do(ctx, store.Request{ID: "register:" + c.nonce + ":" + env.ReqID, Kind: "register", Payload: p.Name},
		func(ctx context.Context, tx *store.Tx) (any, error) {
			id, err := tx.CreateAccount(ctx, p.Name, hash)
			return map[string]int64{"account_id": id}, err
		})
	s.reply(c, env, res, err)
}

func (s *Server) login(ctx context.Context, c *conn, env Envelope) bool {
	if s.accountOf(c) != 0 {
		c.sendError(env.ReqID, ErrAlreadyAuth, "connection already logged in", nil)
		return true
	}
	var p credentials
	if !decode(c, env, &p) {
		return true
	}
	id, hash, err := s.st.Credentials(ctx, p.Name)
	ok := false
	switch {
	case errors.Is(err, store.ErrNotFound):
		auth.VerifyMissing(p.Password)
	case err != nil:
		s.internal(c, env, err)
		return true
	default:
		if ok, err = auth.VerifyPassword(hash, p.Password); err != nil {
			s.internal(c, env, err)
			return true
		}
	}
	if !ok {
		return s.failAuth(c, env)
	}
	token, tokenHash, err := auth.NewToken()
	if err == nil {
		err = s.st.CreateSession(ctx, id, tokenHash, s.cfg.SessionTTL)
	}
	if err != nil {
		s.internal(c, env, err)
		return true
	}
	s.attach(c, id)
	c.send("login.ok", env.ReqID, map[string]any{"account_id": id, "token": token, "expires_in": int(s.cfg.SessionTTL.Seconds())})
	return true
}

func (s *Server) resume(ctx context.Context, c *conn, env Envelope) bool {
	if s.accountOf(c) != 0 {
		c.sendError(env.ReqID, ErrAlreadyAuth, "connection already logged in", nil)
		return true
	}
	var p struct {
		Token string `json:"token"`
	}
	if !decode(c, env, &p) {
		return true
	}
	id, err := s.st.SessionAccount(ctx, auth.TokenHash(p.Token))
	if errors.Is(err, store.ErrNotFound) {
		return s.failAuth(c, env)
	}
	if err != nil {
		s.internal(c, env, err)
		return true
	}
	s.attach(c, id)
	c.send("resume.ok", env.ReqID, map[string]int64{"account_id": id})
	return true
}

// failAuth answers wrong names, passwords and tokens identically and closes
// the connection after too many failures.
func (s *Server) failAuth(c *conn, env Envelope) bool {
	c.failures++
	c.sendError(env.ReqID, ErrBadCredentials, "name, password or token not accepted", nil)
	if c.failures >= s.cfg.MaxLoginFailures {
		c.closeWith(CloseTooManyFailures, "too many failed logins")
		return false
	}
	return true
}

func (s *Server) charList(ctx context.Context, c *conn, env Envelope, account int64) {
	chars, err := s.st.Characters(ctx, account)
	if err != nil {
		s.internal(c, env, err)
		return
	}
	var present any
	if id, state, ok := s.presence.Current(account); ok {
		present = map[string]any{"character_id": id, "state": state}
	}
	if chars == nil {
		chars = []store.Character{}
	}
	c.send("char_list.ok", env.ReqID, map[string]any{"characters": chars, "present": present})
}

func (s *Server) charCreate(ctx context.Context, c *conn, env Envelope, account int64) {
	var p struct {
		Name string `json:"name"`
	}
	if !decode(c, env, &p) {
		return
	}
	if auth.ValidName(p.Name) != nil {
		c.sendError(env.ReqID, ErrInvalidName, "name must be 2-32 letters, digits, _ or -", nil)
		return
	}
	res, err := s.st.Do(ctx, store.Request{ID: fmt.Sprintf("a%d:%s", account, env.ReqID), AccountID: account, Kind: "char_create", Payload: p.Name},
		func(ctx context.Context, tx *store.Tx) (any, error) {
			id, _, err := tx.CreateCharacter(ctx, account, p.Name)
			return store.Character{ID: id, Name: p.Name}, err
		})
	s.reply(c, env, res, err)
}

func (s *Server) enterWorld(ctx context.Context, c *conn, env Envelope, account int64) {
	var p struct {
		CharacterID int64 `json:"character_id"`
	}
	if !decode(c, env, &p) {
		return
	}
	info, err := s.st.CharacterInfo(ctx, p.CharacterID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && info.AccountID != account) {
		c.sendError(env.ReqID, ErrUnknownCharacter, "no such character on this account", nil)
		return
	}
	if err != nil {
		s.internal(c, env, err)
		return
	}
	var other *session.OtherCharacterPresent
	if err := s.presence.Enter(account, p.CharacterID, c.id); errors.As(err, &other) {
		c.sendError(env.ReqID, ErrCharacterPresent, "another character of this account is still in the world; reconnect it or wait until it leaves",
			map[string]int64{"character_id": other.CharacterID})
		return
	}
	// A lingering or kicked-away character is still in the world: take it over.
	pos, area, ok := s.world.Attach(p.CharacterID, c)
	if !ok {
		pos, area = s.world.Spawn(p.CharacterID, info.Name, world.Point{info.X, info.Z}, info.Dead, c)
	}
	c.character = p.CharacterID
	v, _ := s.world.Get(p.CharacterID)
	c.send("enter_world.ok", env.ReqID, map[string]any{"character_id": p.CharacterID, "name": info.Name,
		"x": pos[0], "z": pos[1], "area": area.ID, "zone": area.Zone, "dead": v.Dead, "map": s.cfg.Map})
}

// leaveWorld is an active logout: immediate when out of combat in a safe
// zone; otherwise the character stays until it has disengaged and lingered
// (a "left_world" push follows), and acting or being attacked cancels it.
func (s *Server) leaveWorld(c *conn, env Envelope, account int64) {
	if c.character == 0 {
		c.sendError(env.ReqID, ErrNotInWorld, "this connection has no character in the world", nil)
		return
	}
	immediate, err := s.world.RequestLeave(c.character)
	if err != nil {
		s.worldError(c, env, err)
		return
	}
	if !immediate {
		c.send("leave_world.ok", env.ReqID, map[string]bool{"left": false, "delayed": true})
		return
	}
	if err := s.presence.Leave(account, c.id); err != nil {
		c.sendError(env.ReqID, ErrNotInWorld, "this connection has no character in the world", nil)
		return
	}
	c.character = 0
	c.send("leave_world.ok", env.ReqID, map[string]bool{"left": true})
}

func (s *Server) moveTo(c *conn, env Envelope) {
	var p struct {
		X          *float64 `json:"x"`
		Z          *float64 `json:"z"`
		AcceptRisk bool     `json:"accept_risk"`
	}
	if !decode(c, env, &p) {
		return
	}
	if p.X == nil || p.Z == nil {
		c.sendError(env.ReqID, ErrBadRequest, "move_to needs x and z", nil)
		return
	}
	if c.character == 0 {
		c.sendError(env.ReqID, ErrNotInWorld, "enter the world first", nil)
		return
	}
	path, err := s.world.MoveTo(c.character, world.Point{*p.X, *p.Z}, p.AcceptRisk, time.Now())
	switch {
	case errors.Is(err, world.ErrRiskRequired):
		c.sendError(env.ReqID, ErrRiskConfirmation, "the route enters a risk area where other players may attack; resend with accept_risk", nil)
	case errors.Is(err, world.ErrUnreachable):
		c.sendError(env.ReqID, ErrUnreachable, "no walkable route to that point", nil)
	case errors.Is(err, world.ErrRateLimited):
		c.sendError(env.ReqID, ErrRateLimited, "too many move requests", nil)
	case errors.Is(err, world.ErrNotInWorld):
		c.sendError(env.ReqID, ErrNotInWorld, "enter the world first", nil)
	case errors.Is(err, world.ErrBusy):
		c.sendError(env.ReqID, ErrBusy, "cannot move while acting, stunned or dead", nil)
	case err != nil:
		s.internal(c, env, err)
	default:
		c.send("move_to.ok", env.ReqID, map[string]any{"path": path})
	}
}

func (s *Server) stop(c *conn, env Envelope) {
	if c.character == 0 || s.world.Stop(c.character) != nil {
		c.sendError(env.ReqID, ErrNotInWorld, "enter the world first", nil)
		return
	}
	c.send("stop.ok", env.ReqID, nil)
}

// reply turns a store result (fresh or replayed) into a response.
func (s *Server) reply(c *conn, env Envelope, res store.Result, err error) {
	switch {
	case errors.Is(err, store.ErrRequestMismatch):
		c.sendError(env.ReqID, ErrBadRequest, "req_id already used for a different request", nil)
	case err != nil:
		s.internal(c, env, err)
	case !res.OK:
		code := map[string]string{store.CodeNameTaken: ErrNameTaken, store.CodeCharacterLimit: ErrCharacterLimit}[res.Rejection.Code]
		if code == "" {
			code = ErrBadRequest
		}
		c.sendError(env.ReqID, code, res.Rejection.Code, nil)
	default:
		c.send(env.Type+".ok", env.ReqID, res.Data)
	}
}
