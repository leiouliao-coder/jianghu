// Package store is the PostgreSQL asset ledger: migrations, idempotent
// request transactions and the item/coin operations that run inside them
// (baseline §4.1). Every write goes through Store.Do with a request ID; a
// replayed ID returns the first recorded result and changes nothing.
package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/ledger"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrRequestMismatch = errors.New("store: request id reused with different account, kind or payload")
	ErrBadRequest      = errors.New("store: invalid request")
)

// Rejection is a rule refusal. It is recorded under the request ID like a
// success, and all changes made before it are rolled back.
type Rejection struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

func (r *Rejection) Error() string { return "rejected: " + r.Code + " " + r.Detail }

func reject(code, format string, args ...any) error {
	return &Rejection{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Rejection codes.
const (
	CodeNotHere           = "not_here"
	CodeNoSlots           = "no_slots"
	CodeOverweight        = "overweight"
	CodeInsufficientCoins = "insufficient_coins"
	CodeCharacterLimit    = "character_limit"
	CodeInvalid           = "invalid"
	CodeNameTaken         = "name_taken"
)

type Store struct {
	pool  *pgxpool.Pool
	data  *gamedata.Data
	rules ledger.Rules
}

func Open(ctx context.Context, url string, d *gamedata.Data) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, data: d, rules: ledger.NewRules(d)}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Migrate applies embedded migrations in order. An advisory lock serializes
// concurrent server starts.
func (s *Store) Migrate(ctx context.Context) error {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7419001)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
			version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
			return err
		}
		for _, name := range names {
			base := strings.TrimPrefix(name, "migrations/")
			version, err := strconv.Atoi(strings.SplitN(base, "_", 2)[0])
			if err != nil {
				return fmt.Errorf("migration %s: bad version prefix", base)
			}
			var done bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&done); err != nil {
				return err
			}
			if done {
				continue
			}
			sql, err := migrations.ReadFile(name)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("migration %s: %w", base, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
				return err
			}
		}
		return nil
	})
}

// Request identifies one client intent. AccountID 0 means a system request.
type Request struct {
	ID        string
	AccountID int64
	Kind      string
	Payload   any
}

// Result is what a request produced; replays return the same Result with
// Replayed set.
type Result struct {
	OK        bool            `json:"ok"`
	Data      json.RawMessage `json:"data,omitempty"`
	Rejection *Rejection      `json:"rejection,omitempty"`
	Replayed  bool            `json:"-"`
}

// Do runs fn once per request ID. Transient database errors roll back
// without recording, so the client may retry with the same ID.
func (s *Store) Do(ctx context.Context, req Request, fn func(ctx context.Context, tx *Tx) (any, error)) (Result, error) {
	if req.ID == "" || len(req.ID) > 128 || req.Kind == "" {
		return Result{}, ErrBadRequest
	}
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return Result{}, fmt.Errorf("%w: payload: %v", ErrBadRequest, err)
	}
	sum := sha256.Sum256(payload)
	if res, found, err := s.lookup(ctx, req, sum[:]); found || err != nil {
		return res, err
	}
	var res Result
	err = pgx.BeginFunc(ctx, s.pool, func(ptx pgx.Tx) error {
		res = Result{}
		var data any
		// The savepoint lets a rejection discard fn's partial writes while
		// still recording the outcome under this request ID.
		ferr := pgx.BeginFunc(ctx, ptx, func(sp pgx.Tx) error {
			var err error
			data, err = fn(ctx, &Tx{tx: sp, store: s, requestID: req.ID})
			return err
		})
		var rej *Rejection
		switch {
		case errors.As(ferr, &rej):
			res.Rejection = rej
		case ferr != nil:
			return ferr
		default:
			res.OK = true
			if res.Data, err = json.Marshal(data); err != nil {
				return err
			}
		}
		stored, err := json.Marshal(res)
		if err != nil {
			return err
		}
		outcome := "ok"
		if !res.OK {
			outcome = "rejected"
		}
		var account *int64
		if req.AccountID != 0 {
			account = &req.AccountID
		}
		_, err = ptx.Exec(ctx, `INSERT INTO requests (request_id, account_id, kind, payload_sha256, outcome, result)
			VALUES ($1, $2, $3, $4, $5, $6)`, req.ID, account, req.Kind, sum[:], outcome, stored)
		return err
	})
	if isUniqueViolation(err, "requests_pkey") {
		// A concurrent duplicate committed first; our attempt was rolled back.
		res, found, lerr := s.lookup(ctx, req, sum[:])
		if !found && lerr == nil {
			lerr = errors.New("store: request vanished after conflict")
		}
		return res, lerr
	}
	return res, err
}

func (s *Store) lookup(ctx context.Context, req Request, sum []byte) (Result, bool, error) {
	var (
		account *int64
		kind    string
		hash    []byte
		stored  []byte
	)
	err := s.pool.QueryRow(ctx, `SELECT account_id, kind, payload_sha256, result FROM requests WHERE request_id = $1`, req.ID).
		Scan(&account, &kind, &hash, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	var owner int64
	if account != nil {
		owner = *account
	}
	if owner != req.AccountID || kind != req.Kind || string(hash) != string(sum) {
		return Result{}, true, ErrRequestMismatch
	}
	var res Result
	if err := json.Unmarshal(stored, &res); err != nil {
		return Result{}, true, err
	}
	res.Replayed = true
	return res, true, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && (constraint == "" || pg.ConstraintName == constraint)
}

// Tx is the request-scoped transaction handed to Do callbacks.
type Tx struct {
	tx        pgx.Tx
	store     *Store
	requestID string
}

// Event appends an audit record tied to this request.
func (t *Tx) Event(ctx context.Context, kind string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(ctx, `INSERT INTO events (request_id, kind, data) VALUES ($1, $2, $3)`, t.requestID, kind, b)
	return err
}
