package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CorpseInfo is a live corpse container in the world.
type CorpseInfo struct {
	ID        int64     `json:"id"`
	X         float64   `json:"x"`
	Z         float64   `json:"z"`
	OwnerName string    `json:"owner_name"`
	ExpiresAt time.Time `json:"expires_at"`
	Empty     bool      `json:"empty"`
}

// CorpseItem is one stack inside a corpse.
type CorpseItem struct {
	ID    string `json:"id"`
	Proto string `json:"proto"`
	State string `json:"state"`
	Qty   int64  `json:"qty"`
}

// DropAll creates a corpse at the death position and moves every item stack
// and all coins from the backpack into it, in one transaction. Death never
// touches permanent growth; only carried goods drop.
func (t *Tx) DropAll(ctx context.Context, backpackID int64, ownerName string, x, z float64, expires time.Time) (int64, error) {
	var corpse int64
	if err := t.tx.QueryRow(ctx, `INSERT INTO containers (kind, pos_x, pos_z, owner_name, expires_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		Corpse, x, z, ownerName, expires).Scan(&corpse); err != nil {
		return 0, err
	}
	if err := t.lock(ctx, backpackID, corpse); err != nil {
		return 0, err
	}
	tag, err := t.tx.Exec(ctx, `UPDATE items SET container_id = $2, version = version + 1 WHERE container_id = $1`, backpackID, corpse)
	if err != nil {
		return 0, err
	}
	var coins int64
	if err := t.tx.QueryRow(ctx, `UPDATE containers b SET coins = 0 FROM (SELECT coins FROM containers WHERE id = $1) old
		WHERE b.id = $1 RETURNING old.coins`, backpackID).Scan(&coins); err != nil {
		return 0, err
	}
	if _, err := t.tx.Exec(ctx, `UPDATE containers SET coins = $2 WHERE id = $1`, corpse, coins); err != nil {
		return 0, err
	}
	return corpse, t.Event(ctx, "death_drop", map[string]any{"backpack": backpackID, "corpse": corpse,
		"stacks": tag.RowsAffected(), "coins": coins, "x": x, "z": z})
}

// TakeCoins moves every coin in a live corpse to the backpack; a second
// taker finds none (not_here).
func (t *Tx) TakeCoins(ctx context.Context, corpseID, backpackID int64) (int64, error) {
	if err := t.lock(ctx, corpseID, backpackID); err != nil {
		return 0, err
	}
	var coins int64
	if err := t.tx.QueryRow(ctx, `SELECT coins FROM containers WHERE id = $1 AND kind = 'corpse' AND destroyed_at IS NULL`, corpseID).Scan(&coins); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, reject(CodeNotHere, "corpse %d is gone", corpseID)
		}
		return 0, err
	}
	if coins == 0 {
		return 0, reject(CodeNotHere, "no coins left in corpse %d", corpseID)
	}
	return coins, t.MoveCoins(ctx, corpseID, backpackID, coins)
}

// DestroyCorpse deletes whatever is left in an expired corpse; the coins
// leave circulation. The container row stays as a record.
func (t *Tx) DestroyCorpse(ctx context.Context, corpseID int64) error {
	if err := t.lock(ctx, corpseID); err != nil {
		return err
	}
	var coins int64
	if err := t.tx.QueryRow(ctx, `UPDATE containers c SET destroyed_at = now(), coins = 0 FROM (SELECT coins FROM containers WHERE id = $1) old
		WHERE c.id = $1 AND c.kind = 'corpse' AND c.destroyed_at IS NULL RETURNING old.coins`, corpseID).Scan(&coins); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return reject(CodeNotHere, "corpse %d already gone", corpseID)
		}
		return err
	}
	tag, err := t.tx.Exec(ctx, `DELETE FROM items WHERE container_id = $1`, corpseID)
	if err != nil {
		return err
	}
	return t.Event(ctx, "corpse_expired", map[string]any{"corpse": corpseID, "stacks_destroyed": tag.RowsAffected(), "coins_destroyed": coins})
}

// LiveCorpses lists corpses that still exist, for loading the world.
func (s *Store) LiveCorpses(ctx context.Context) ([]CorpseInfo, error) {
	rows, err := s.pool.Query(ctx, `SELECT c.id, c.pos_x, c.pos_z, c.owner_name, c.expires_at,
		c.coins = 0 AND NOT EXISTS (SELECT 1 FROM items i WHERE i.container_id = c.id)
		FROM containers c WHERE c.kind = 'corpse' AND c.destroyed_at IS NULL ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[CorpseInfo])
}

// CorpseContents lists what is left in a live corpse.
func (s *Store) CorpseContents(ctx context.Context, corpseID int64) ([]CorpseItem, int64, error) {
	var coins int64
	err := s.pool.QueryRow(ctx, `SELECT coins FROM containers WHERE id = $1 AND kind = 'corpse' AND destroyed_at IS NULL`, corpseID).Scan(&coins)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, proto, state, qty FROM items WHERE container_id = $1 ORDER BY id`, corpseID)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CorpseItem])
	if items == nil {
		items = []CorpseItem{}
	}
	return items, coins, err
}

// Backpack returns a character's backpack container ID.
func (s *Store) Backpack(ctx context.Context, characterID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM containers WHERE kind = 'backpack' AND character_id = $1`, characterID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}
