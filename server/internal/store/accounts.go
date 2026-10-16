package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("store: not found")

// Credentials returns the account ID and password hash for a login name.
// Accounts without a password are reported as not found.
func (s *Store) Credentials(ctx context.Context, name string) (int64, string, error) {
	var id int64
	var hash *string
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM accounts WHERE name = $1`, name).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hash == nil) {
		return 0, "", ErrNotFound
	}
	return id, deref(hash), err
}

// CreateSession stores a session token hash. Expired sessions of the
// account are pruned at the same time.
func (s *Store) CreateSession(ctx context.Context, accountID int64, tokenHash []byte, ttl time.Duration) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE account_id = $1 AND expires_at <= now()`, accountID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sessions (token_sha256, account_id, expires_at) VALUES ($1, $2, now() + $3::interval)`,
			tokenHash, accountID, ttl.String())
		return err
	})
}

// SessionAccount resolves an unexpired session token hash.
func (s *Store) SessionAccount(ctx context.Context, tokenHash []byte) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT account_id FROM sessions WHERE token_sha256 = $1 AND expires_at > now()`, tokenHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

type Character struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Characters lists an account's characters in creation order.
func (s *Store) Characters(ctx context.Context, accountID int64) ([]Character, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM characters WHERE account_id = $1 ORDER BY id`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Character])
}

// CharacterInfo is what entering the world needs.
type CharacterInfo struct {
	AccountID int64
	Name      string
	X, Z      float64
	Dead      bool
}

func (s *Store) CharacterInfo(ctx context.Context, characterID int64) (CharacterInfo, error) {
	var c CharacterInfo
	err := s.pool.QueryRow(ctx, `SELECT account_id, name, pos_x, pos_z, dead FROM characters WHERE id = $1`, characterID).
		Scan(&c.AccountID, &c.Name, &c.X, &c.Z, &c.Dead)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// SaveState records where and whether alive a character left the world.
// Neither is an asset, so this is a plain update, not a request transaction.
func (s *Store) SaveState(ctx context.Context, characterID int64, x, z float64, dead bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE characters SET pos_x = $2, pos_z = $3, dead = $4 WHERE id = $1`, characterID, x, z, dead)
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
