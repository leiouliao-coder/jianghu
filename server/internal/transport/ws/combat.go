package ws

import (
	"context"
	"errors"
	"fmt"

	"github.com/leiouliao-coder/jianghu/server/internal/combat"
	"github.com/leiouliao-coder/jianghu/server/internal/store"
	"github.com/leiouliao-coder/jianghu/server/internal/world"
)

var combatErrors = map[error]string{
	combat.ErrDead: ErrDead, combat.ErrStunned: ErrStunned, combat.ErrBusy: ErrBusy, combat.ErrCooldown: ErrCooldown,
	combat.ErrResources: ErrResources, combat.ErrNoSkill: ErrNoSkill, combat.ErrBadTarget: ErrBadTarget,
	combat.ErrOutOfRange: ErrOutOfRange, combat.ErrNoDirection: ErrNoDirection,
	world.ErrNotInWorld: ErrNotInWorld, world.ErrDead: ErrDead, world.ErrNotDead: ErrNotDead, world.ErrTooFar: ErrTooFar,
}

func (s *Server) worldError(c *conn, env Envelope, err error) {
	for e, code := range combatErrors {
		if errors.Is(err, e) {
			c.sendError(env.ReqID, code, err.Error(), nil)
			return
		}
	}
	s.internal(c, env, err)
}

// combatRequest handles cast, swap, respawn, corpse_open and loot for the
// connection's character.
func (s *Server) combatRequest(ctx context.Context, c *conn, env Envelope, account int64) {
	id := c.character
	switch env.Type {
	case "cast":
		var p struct {
			Slot   string     `json:"slot"`
			Dir    [2]float64 `json:"dir"`
			Target int64      `json:"target"`
		}
		if !decode(c, env, &p) {
			return
		}
		if err := s.world.Cast(id, p.Slot, world.Point(p.Dir), p.Target); err != nil {
			s.worldError(c, env, err)
			return
		}
		c.send("cast.ok", env.ReqID, nil)
	case "swap":
		if err := s.world.Swap(id); err != nil {
			s.worldError(c, env, err)
			return
		}
		c.send("swap.ok", env.ReqID, nil)
	case "respawn":
		at, area, err := s.world.Respawn(id)
		if err != nil {
			s.worldError(c, env, err)
			return
		}
		c.send("respawn.ok", env.ReqID, map[string]any{"x": at[0], "z": at[1], "area": area.ID, "zone": area.Zone})
	case "corpse_open":
		var p struct {
			Corpse int64 `json:"corpse_id"`
		}
		if !decode(c, env, &p) {
			return
		}
		if err := s.world.CanLoot(id, p.Corpse); err != nil {
			s.worldError(c, env, err)
			return
		}
		items, coins, err := s.st.CorpseContents(ctx, p.Corpse)
		if errors.Is(err, store.ErrNotFound) {
			c.sendError(env.ReqID, ErrNotHere, "corpse is gone", nil)
			return
		}
		if err != nil {
			s.internal(c, env, err)
			return
		}
		c.send("corpse_open.ok", env.ReqID, map[string]any{"corpse_id": p.Corpse, "items": items, "coins": coins})
	case "loot":
		s.loot(ctx, c, env, account)
	}
}

// loot takes one item stack or all coins from a corpse in reach. Anyone may
// loot; each stack and the coins transfer exactly once (the asset ledger's
// row locks), and a successful take is permanent ownership.
func (s *Server) loot(ctx context.Context, c *conn, env Envelope, account int64) {
	var p struct {
		Corpse int64  `json:"corpse_id"`
		Item   string `json:"item_id"`
		Coins  bool   `json:"coins"`
	}
	if !decode(c, env, &p) {
		return
	}
	if (p.Item == "") == !p.Coins {
		c.sendError(env.ReqID, ErrBadRequest, "loot needs exactly one of item_id or coins", nil)
		return
	}
	if err := s.world.CanLoot(c.character, p.Corpse); err != nil {
		s.worldError(c, env, err)
		return
	}
	backpack, err := s.st.Backpack(ctx, c.character)
	if err != nil {
		s.internal(c, env, err)
		return
	}
	res, err := s.st.Do(ctx, store.Request{ID: fmt.Sprintf("a%d:%s", account, env.ReqID), AccountID: account, Kind: "loot",
		Payload: map[string]any{"corpse": p.Corpse, "item": p.Item, "coins": p.Coins}},
		func(ctx context.Context, tx *store.Tx) (any, error) {
			if p.Coins {
				n, err := tx.TakeCoins(ctx, p.Corpse, backpack)
				return map[string]int64{"coins": n}, err
			}
			return map[string]string{"item_id": p.Item}, tx.MoveItem(ctx, p.Item, p.Corpse, backpack)
		})
	switch {
	case errors.Is(err, store.ErrRequestMismatch):
		c.sendError(env.ReqID, ErrBadRequest, "req_id already used for a different request", nil)
	case err != nil:
		s.internal(c, env, err)
	case !res.OK:
		code := map[string]string{store.CodeNotHere: ErrNotHere, store.CodeNoSlots: ErrNoSpace, store.CodeOverweight: ErrNoSpace}[res.Rejection.Code]
		if code == "" {
			code = ErrBadRequest
		}
		c.sendError(env.ReqID, code, res.Rejection.Code, nil)
	default:
		if items, coins, err := s.st.CorpseContents(ctx, p.Corpse); err == nil && len(items) == 0 && coins == 0 {
			s.world.CorpseEmptied(p.Corpse)
		}
		c.send("loot.ok", env.ReqID, res.Data)
	}
}
