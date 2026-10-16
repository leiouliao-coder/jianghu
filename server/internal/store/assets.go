package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/growth"
	"github.com/leiouliao-coder/jianghu/server/internal/ledger"
)

// Container kinds; see migrations/0001_init.sql.
const (
	Backpack    = "backpack"
	NodeStorage = "node_storage"
	Corpse      = "corpse"
	NPCStock    = "npc_stock"
	Escrow      = "escrow"
	System      = "system"
)

// raceWindow runs between a rule check and the write that depends on it.
// It is a no-op in production; concurrency tests make it sleep so racing
// requests reliably overlap and a missing lock shows up as a failure.
var raceWindow = func(ctx context.Context, tx pgx.Tx) {}

func newItemID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}

// CreateAccount creates an account. An empty passwordHash makes an account
// that cannot log in (system and test accounts).
func (t *Tx) CreateAccount(ctx context.Context, name, passwordHash string) (int64, error) {
	var id int64
	err := t.tx.QueryRow(ctx, `INSERT INTO accounts (name, password_hash) VALUES ($1, NULLIF($2, '')) ON CONFLICT (name) DO NOTHING RETURNING id`,
		name, passwordHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, reject(CodeNameTaken, "account %q", name)
	}
	if err != nil {
		return 0, err
	}
	return id, t.Event(ctx, "account_created", map[string]any{"account": id})
}

// CreateCharacter enforces the per-account character limit under the
// account row lock and gives the character its backpack.
func (t *Tx) CreateCharacter(ctx context.Context, accountID int64, name string) (charID, backpackID int64, err error) {
	var count int
	if err := t.tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&accountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, reject(CodeInvalid, "account %d does not exist", accountID)
		}
		return 0, 0, err
	}
	if err := t.tx.QueryRow(ctx, `SELECT count(*) FROM characters WHERE account_id = $1`, accountID).Scan(&count); err != nil {
		return 0, 0, err
	}
	if limit := t.store.data.World.Characters.PerAccount; count >= limit {
		return 0, 0, reject(CodeCharacterLimit, "account already has %d of %d characters", count, limit)
	}
	err = t.tx.QueryRow(ctx, `INSERT INTO characters (account_id, name) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING RETURNING id`,
		accountID, name).Scan(&charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, reject(CodeNameTaken, "character %q", name)
	}
	if err != nil {
		return 0, 0, err
	}
	if err := t.tx.QueryRow(ctx, `INSERT INTO containers (kind, account_id, character_id, slot_cap) VALUES ($1, $2, $3, $4) RETURNING id`,
		Backpack, accountID, charID, t.store.data.World.Inventory.BackpackSlots).Scan(&backpackID); err != nil {
		return 0, 0, err
	}
	return charID, backpackID, t.Event(ctx, "character_created", map[string]any{"account": accountID, "character": charID, "backpack": backpackID})
}

// NodeStorage returns the account's shared storage at a logistics node,
// creating it on first use.
func (t *Tx) NodeStorage(ctx context.Context, accountID int64, node string) (int64, error) {
	if _, err := t.tx.Exec(ctx, `INSERT INTO containers (kind, account_id, node, slot_cap) VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id, node) WHERE kind = 'node_storage' DO NOTHING`,
		NodeStorage, accountID, node, t.store.data.World.Inventory.NodeSlotsPerAccount); err != nil {
		return 0, err
	}
	var id int64
	err := t.tx.QueryRow(ctx, `SELECT id FROM containers WHERE kind = $1 AND account_id = $2 AND node = $3`, NodeStorage, accountID, node).Scan(&id)
	return id, err
}

// NewContainer creates a corpse, NPC stock, escrow or system container.
// slotCap nil means no slot limit.
func (t *Tx) NewContainer(ctx context.Context, kind string, node string, slotCap *int) (int64, error) {
	switch kind {
	case Backpack, NodeStorage, Corpse:
		return 0, reject(CodeInvalid, "use CreateCharacter, NodeStorage or DropAll for %s", kind)
	}
	var id int64
	err := t.tx.QueryRow(ctx, `INSERT INTO containers (kind, node, slot_cap) VALUES ($1, NULLIF($2, ''), $3) RETURNING id`, kind, node, slotCap).Scan(&id)
	return id, err
}

// Mint creates a new item stack from outside the economy (system grants,
// tests). Normal play moves existing items instead.
func (t *Tx) Mint(ctx context.Context, containerID int64, proto, state string, qty int64) (string, error) {
	it, ok := t.store.data.Items[proto]
	if !ok || it.Class == gamedata.ClassCoin || qty <= 0 || qty > t.store.data.StackLimits[it.Class] {
		return "", reject(CodeInvalid, "cannot mint %d x %q in one stack", qty, proto)
	}
	if err := t.lock(ctx, containerID); err != nil {
		return "", err
	}
	if err := t.fits(ctx, containerID, []ledger.Stack{{Item: proto, State: state, Qty: qty}}, 0); err != nil {
		return "", err
	}
	raceWindow(ctx, t.tx)
	id := newItemID()
	if _, err := t.tx.Exec(ctx, `INSERT INTO items (id, proto, state, qty, container_id, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
		id, proto, state, qty, containerID, t.requestID); err != nil {
		return "", err
	}
	return id, t.Event(ctx, "item_minted", map[string]any{"item": id, "proto": proto, "qty": qty, "container": containerID})
}

// MintCoins adds coins from outside the economy (system grants, tests).
func (t *Tx) MintCoins(ctx context.Context, containerID, amount int64) error {
	if amount <= 0 {
		return reject(CodeInvalid, "amount must be positive")
	}
	if err := t.lock(ctx, containerID); err != nil {
		return err
	}
	if err := t.fits(ctx, containerID, nil, amount); err != nil {
		return err
	}
	if _, err := t.tx.Exec(ctx, `UPDATE containers SET coins = coins + $2 WHERE id = $1`, containerID, amount); err != nil {
		return err
	}
	return t.Event(ctx, "coins_minted", map[string]any{"container": containerID, "amount": amount})
}

// MoveItem moves a whole stack. The item must still be in `from`: of many
// concurrent movers exactly one succeeds, the rest get not_here.
func (t *Tx) MoveItem(ctx context.Context, itemID string, from, to int64) error {
	if from == to {
		return reject(CodeInvalid, "same container")
	}
	if err := t.lock(ctx, from, to); err != nil {
		return err
	}
	var s ledger.Stack
	err := t.tx.QueryRow(ctx, `SELECT proto, state, qty FROM items WHERE id = $1 AND container_id = $2 FOR UPDATE`, itemID, from).
		Scan(&s.Item, &s.State, &s.Qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return reject(CodeNotHere, "item %s is not in container %d", itemID, from)
	}
	if err != nil {
		return err
	}
	if err := t.fits(ctx, to, []ledger.Stack{s}, 0); err != nil {
		return err
	}
	raceWindow(ctx, t.tx)
	if _, err := t.tx.Exec(ctx, `UPDATE items SET container_id = $2, version = version + 1 WHERE id = $1`, itemID, to); err != nil {
		return err
	}
	return t.Event(ctx, "item_moved", map[string]any{"item": itemID, "from": from, "to": to})
}

// MoveCoins transfers coins between containers at the caller's location; it
// never creates a balance anywhere else.
func (t *Tx) MoveCoins(ctx context.Context, from, to, amount int64) error {
	if amount <= 0 || from == to {
		return reject(CodeInvalid, "amount must be positive and containers distinct")
	}
	if err := t.lock(ctx, from, to); err != nil {
		return err
	}
	tag, err := t.tx.Exec(ctx, `UPDATE containers SET coins = coins - $2 WHERE id = $1 AND coins >= $2`, from, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return reject(CodeInsufficientCoins, "container %d has fewer than %d coins", from, amount)
	}
	if err := t.fits(ctx, to, nil, amount); err != nil {
		return err
	}
	raceWindow(ctx, t.tx)
	if _, err := t.tx.Exec(ctx, `UPDATE containers SET coins = coins + $2 WHERE id = $1`, to, amount); err != nil {
		return err
	}
	return t.Event(ctx, "coins_moved", map[string]any{"from": from, "to": to, "amount": amount})
}

// lock takes container row locks in ascending ID order so concurrent
// transfers cannot deadlock.
func (t *Tx) lock(ctx context.Context, ids ...int64) error {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	rows, err := t.tx.Query(ctx, `SELECT id FROM containers WHERE id = ANY($1) ORDER BY id FOR UPDATE`, ids)
	if err != nil {
		return err
	}
	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if n != len(slices.Compact(ids)) {
		return reject(CodeInvalid, "unknown container in %v", ids)
	}
	return nil
}

// fits checks slots (one per item stack plus coin stacks) and, for
// backpacks, carry weight. Moves never auto-merge stacks: merging is a
// separate local action, so every incoming stack takes a slot.
// The container must already be locked.
func (t *Tx) fits(ctx context.Context, containerID int64, add []ledger.Stack, addCoins int64) error {
	var (
		kind      string
		slotCap   *int
		coins     int64
		character *int64
	)
	if err := t.tx.QueryRow(ctx, `SELECT kind, slot_cap, coins, character_id FROM containers WHERE id = $1`, containerID).
		Scan(&kind, &slotCap, &coins, &character); err != nil {
		return err
	}
	rows, err := t.tx.Query(ctx, `SELECT proto, state, qty FROM items WHERE container_id = $1`, containerID)
	if err != nil {
		return err
	}
	held, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ledger.Stack, error) {
		var s ledger.Stack
		return s, r.Scan(&s.Item, &s.State, &s.Qty)
	})
	if err != nil {
		return err
	}
	coinItem := t.store.data.ItemsOfClass(gamedata.ClassCoin)[0]
	coinStack := t.store.data.StackLimits[gamedata.ClassCoin]
	if slotCap != nil {
		after := len(held) + len(add) + int((coins+addCoins+coinStack-1)/coinStack)
		if after > *slotCap {
			return reject(CodeNoSlots, "container %d needs %d of %d slots", containerID, after, *slotCap)
		}
	}
	if kind != Backpack {
		return nil
	}
	var strength growth.Centi
	if err := t.tx.QueryRow(ctx, `SELECT strength_centi FROM characters WHERE id = $1`, *character).Scan(&strength); err != nil {
		return err
	}
	if coins > 0 {
		held = append(held, ledger.Stack{Item: coinItem, Qty: coins})
	}
	if addCoins > 0 {
		add = append(add, ledger.Stack{Item: coinItem, Qty: addCoins})
	}
	have, err := t.store.rules.WeightCenti(held)
	if err != nil {
		return err
	}
	extra, err := t.store.rules.WeightCenti(add)
	if err != nil {
		return err
	}
	if limit := ledger.CarryLimitCenti(strength); have+extra > limit {
		return reject(CodeOverweight, "carry %d+%d over %d (1/100 units)", have, extra, limit)
	}
	return nil
}
