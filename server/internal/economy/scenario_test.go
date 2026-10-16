package economy

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/leiouliao-coder/jianghu/server/internal/crafting"
	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
)

// Re-runs every scenario of verify_economy_v44.py with the Go order book and
// crafting rules, and requires each snapshot to equal economy_v44_run.json.
// The wallets, goods and material stocks below are a test harness standing in
// for the M1/M5 ledger; the rules under test are Book and crafting.
const goldenPath = "../../../docs/design/v0.44/economy_v44_run.json"

type wallet struct{ Carry, Pending, Bank int64 }

type good struct {
	owner, node, name, state string
	eq                       *crafting.Equipment
}

type world struct {
	t           *testing.T
	d           *gamedata.Data
	b           *Book
	money       map[string]map[string]*wallet
	goods       map[string]*good
	material    map[[2]string]crafting.Stock
	burn, moved int64
	initial     int64
	logs        []any
}

func params(d *gamedata.Data) Params {
	e := d.Economy
	return Params{PoolInitial: e.Pool.Initial, PoolCap: e.Pool.Cap, HourlyRefill: e.Pool.HourlyRefill,
		AccountLimit: e.AccountLimit.Amount, WindowHours: e.AccountLimit.WindowHours,
		NodeSlots: e.NodeSlotsPerAccount, Rewards: e.OrderRewards}
}

func newWorld(t *testing.T, d *gamedata.Data, players map[string]int64) *world {
	w := &world{t: t, d: d, b: NewBook(params(d), "A", "B"), money: map[string]map[string]*wallet{},
		goods: map[string]*good{}, material: map[[2]string]crafting.Stock{}}
	for a, v := range players {
		w.money[a] = map[string]*wallet{"A": {Carry: v}, "B": {}}
	}
	w.initial = w.total()
	return w
}

func (w *world) total() int64 {
	t := w.b.PoolsTotal()
	for _, nodes := range w.money {
		for _, m := range nodes {
			t += m.Carry + m.Pending + m.Bank
		}
	}
	return t
}

func (w *world) must(err error, what string) {
	w.t.Helper()
	if err != nil {
		w.t.Fatalf("%s: %v", what, err)
	}
	w.invariant()
}

func (w *world) invariant() {
	w.t.Helper()
	if got, want := w.total(), w.initial+w.b.Mint()-w.burn; got != want {
		w.t.Fatalf("money conservation: total %d, want %d", got, want)
	}
	if err := w.b.Check(); err != nil {
		w.t.Fatal(err)
	}
	for _, n := range w.b.DemandNodes() {
		for item := range w.b.Need(n) {
			held := 0
			for _, g := range w.goods {
				if g.node == n && g.name == item && g.state == "npc-stock" {
					held++
				}
			}
			if held != w.b.StockCount(n, item) {
				w.t.Fatalf("NPC stock %s/%s = %d, actual goods %d", n, item, w.b.StockCount(n, item), held)
			}
		}
	}
	for a, nodes := range w.money {
		for n, m := range nodes {
			if m.Carry < 0 || m.Pending < 0 || m.Bank < 0 {
				w.t.Fatalf("negative money %s/%s", a, n)
			}
		}
	}
}

func (w *world) spec(name string) gamedata.Equipment {
	eq := w.d.EquipmentByName(name)
	if eq == nil {
		w.t.Fatalf("no equipment %q", name)
	}
	return *eq
}

func (w *world) buy(a, n string, mats map[string]int64) {
	var price int64
	for k, v := range mats {
		price += w.d.Economy.MaterialPrices[k] * v
	}
	if w.money[a][n].Carry < price {
		w.t.Fatal("buy: local money insufficient")
	}
	w.money[a][n].Carry -= price
	w.burn += price
	st := w.stock(a, n)
	for k, v := range mats {
		st[k] += v
	}
	w.invariant()
}

func (w *world) stock(a, n string) crafting.Stock {
	k := [2]string{a, n}
	if w.material[k] == nil {
		w.material[k] = crafting.Stock{}
	}
	return w.material[k]
}

func (w *world) craft(a, n string, spec gamedata.Equipment, id string) *good {
	if w.goods[id] != nil {
		w.t.Fatalf("item id %s reused", id)
	}
	eq, err := crafting.Manufacture(w.stock(a, n), spec, id)
	if err != nil {
		w.t.Fatalf("craft %s: %v", id, err)
	}
	g := &good{owner: a, node: n, name: spec.Name, state: "player", eq: eq}
	w.goods[id] = g
	return g
}

func (w *world) lifecycle(a, n string, spec gamedata.Equipment, id string, wear []int64) []any {
	g := w.craft(a, n, spec, id)
	st := func(stage string) any {
		return map[string]any{"stage": stage, "current": g.eq.Durability, "max": g.eq.MaxDurability}
	}
	trace := []any{st("made")}
	for i, x := range wear {
		w.must(g.eq.Wear(x), "wear")
		trace = append(trace, st("wear"))
		if i < 2 {
			w.must(crafting.Repair(w.stock(a, n), g.eq, spec, spec.Profession, w.d.RepairMaxLoss), "repair")
			trace = append(trace, st("repair"))
		}
	}
	if g.eq.Durability != 0 || g.eq.MaxDurability != 96 {
		w.t.Fatalf("two-repair wear result %+v", g.eq)
	}
	w.must(crafting.Salvage(w.stock(a, n), g.eq, spec, true), "salvage")
	g.state = "recycled"
	return append(trace, map[string]any{"stage": "recycled", "salvage": spec.Salvage})
}

func (w *world) deliver(oid, iid string) error {
	g := w.goods[iid]
	goods := Goods{ID: iid}
	if g != nil {
		full := w.spec(g.name).MaxDurability
		goods = Goods{ID: iid, Owner: g.owner, Node: g.node, Name: g.name,
			Pristine: g.state == "player" && g.eq.Durability == full && g.eq.MaxDurability == full}
	}
	p, err := w.b.Deliver(oid, goods)
	if err == nil {
		g.state = "npc-stock"
		w.money[p.Account][p.Node].Pending += p.Amount
	}
	w.invariant()
	return err
}

func (w *world) consume(n, item string, ids ...string) {
	if err := w.b.Consume(n, item, ids); err != nil {
		w.t.Fatalf("consume: %v", err)
	}
	for _, id := range ids {
		w.goods[id].state = "npc-consumed"
	}
	w.invariant()
}

func (w *world) claimBank(a, n string) {
	m := w.money[a][n]
	m.Bank, m.Pending = m.Bank+m.Pending, 0
	w.invariant()
}

func (w *world) moveMoney(a, b, n string, amount int64) {
	if w.money[a][n].Carry < amount {
		w.t.Fatal("transfer balance")
	}
	w.money[a][n].Carry -= amount
	w.money[b][n].Carry += amount
	w.moved += amount
	w.invariant()
}

func (w *world) snap(label string) {
	w.invariant()
	nodes := map[string]any{}
	for _, n := range w.b.Nodes() {
		f := w.b.Frozen(n)
		nodes[n] = map[string]int64{"T": w.b.Pool(n), "F": f, "available": w.b.Pool(n) - f}
	}
	stock, need := map[string]any{}, map[string]any{}
	for _, n := range w.b.DemandNodes() {
		s := map[string]int{}
		for item := range w.b.Need(n) {
			s[item] = w.b.StockCount(n, item)
		}
		stock[n], need[n] = s, w.b.Need(n)
	}
	orders := map[string]string{}
	for _, id := range w.b.OrderIDs() {
		orders[id] = string(w.b.Order(id).State)
	}
	goods := map[string]string{}
	for id, g := range w.goods {
		goods[id] = g.state
	}
	money := map[string]any{}
	for a, ns := range w.money {
		m := map[string]any{}
		for n, x := range ns {
			m[n] = map[string]int64{"carry": x.Carry, "pending": x.Pending, "bank": x.Bank}
		}
		money[a] = m
	}
	hour, real := w.b.Hours()
	w.logs = append(w.logs, map[string]any{"label": label, "hour": hour, "real_hour": real, "nodes": nodes,
		"player_money": money, "world": w.total(), "mint": w.b.Mint(), "burn": w.burn, "npc_paid": w.b.NPCPaid(),
		"player_transfer": w.moved, "demand_stock": stock, "need": need, "orders": orders, "goods_states": goods})
}

func is(t *testing.T, err, want error, what string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: err=%v, want %v", what, err, want)
	}
}

// normalize round-trips through JSON so Go values compare with decoded golden data.
func normalize(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func compare(t *testing.T, name string, got, want any) {
	t.Helper()
	g := normalize(t, got)
	if !reflect.DeepEqual(g, want) {
		gb, _ := json.MarshalIndent(g, "", " ")
		wb, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("%s differs from golden\n got: %s\nwant: %s", name, gb, wb)
	}
}

func TestEconomyScenariosMatchGolden(t *testing.T) {
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]any
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	sword, cloth := w0spec(t, d, "测试铁剑"), w0spec(t, d, "测试布护具")
	wear := []int64{65, 70, 96}

	// Prices: lifecycle cost and order margin per item.
	var prices []any
	for _, r := range []gamedata.Equipment{sword, cloth} {
		cost := func(m map[string]int64) (c int64) {
			for k, v := range m {
				c += d.Economy.MaterialPrices[k] * v
			}
			return
		}
		fresh, repair, reward := cost(r.Manufacture), cost(r.Repair), d.Economy.OrderRewards[r.Name]
		prices = append(prices, map[string]any{"item": r.Name, "fresh_cost": fresh, "repair_cost": repair,
			"lifecycle_cost": fresh + 2*repair, "order_reward": reward, "order_net_before_labor": reward - fresh})
	}
	compare(t, "prices", prices, golden["prices"])

	// A: one account, two nodes; 200 coin was physically carried to B first.
	scenarioA := func() (*world, map[string]any) {
		a := newWorld(t, d, map[string]int64{"solo": 1000})
		a.money["solo"]["A"].Carry, a.money["solo"]["B"].Carry = 800, 200
		a.snap("A0 已沿普通道路实际携款200抵B；存量1000非本期发行")
		a.b.SetDemand("A", sword.Name, 1)
		a.b.SetDemand("B", cloth.Name, 1)
		traces := map[string]any{}
		for _, x := range []struct {
			node, prefix string
			r            gamedata.Equipment
		}{{"A", "sword", sword}, {"B", "cloth", cloth}} {
			// Initial lifecycle materials that have an NPC price, plus one more recipe for the order piece.
			both := map[string]int64{}
			for k, v := range ledgerInitial(t, x.r.ID) {
				if _, priced := d.Economy.MaterialPrices[k]; priced {
					both[k] += v
				}
			}
			for k, v := range x.r.Manufacture {
				both[k] += v
			}
			a.buy("solo", x.node, both)
			a.craft("solo", x.node, x.r, x.prefix+"-ORDER")
			traces[x.prefix] = a.lifecycle("solo", x.node, x.r, x.prefix+"-SELF", wear)
		}
		a.snap("A1 购料并制造独立订单件、自用件，两修回收完成")
		a.must(a.b.Accept("s", "solo", "A", sword.Name), "sword order")
		a.must(a.b.Accept("c", "solo", "B", cloth.Name), "cloth order")
		a.snap("A2 接单冻结350，账号预占350")
		a.b.Refill(1)
		a.snap("A3 运行1小时两池各发行200")
		a.must(a.deliver("s", "sword-ORDER"), "sword deliver")
		a.must(a.deliver("c", "cloth-ORDER"), "cloth deliver")
		a.snap("A4 交付：本地待领A200/B150，收入350")
		a.claimBank("solo", "A")
		a.claimBank("solo", "B")
		a.snap("A5 同节点领取至银行，不跨节点不重复占额")
		return a, traces
	}
	a, traces := scenarioA()
	var player int64
	for _, m := range a.money["solo"] {
		player += m.Carry + m.Pending + m.Bank
	}
	if a.burn != 340 || a.b.Mint() != 400 || a.total() != 3060 || player != 1010 {
		t.Fatalf("A totals burn=%d mint=%d world=%d player=%d", a.burn, a.b.Mint(), a.total(), player)
	}
	compare(t, "A", a.logs, golden["A"])
	compare(t, "lifecycle", traces, golden["lifecycle"])

	// B: serialized competition for the last unit of demand.
	b := newWorld(t, d, map[string]int64{"p": 1000, "q": 1000})
	b.b.SetDemand("A", sword.Name, 1)
	b.snap("B0 请求顺序p先q后")
	b.must(b.b.Accept("p1", "p", "A", sword.Name), "first accepted")
	before := b.total()
	is(t, b.b.Accept("q1", "q", "A", sword.Name), ErrNoDemand, "same last demand")
	if b.total() != before {
		t.Fatal("rejection changed money")
	}
	b.snap("B1 p锁定唯一需求，q被拒")
	b.buy("p", "A", sword.Manufacture)
	b.craft("p", "A", sword, "B-S")
	b.must(b.deliver("p1", "B-S"), "deliver")
	before = b.total()
	is(t, b.deliver("p1", "B-S"), ErrAlreadyPaid, "replay")
	if b.total() != before {
		t.Fatal("replay paid twice")
	}
	b.snap("B2 p收入200；重放未重复支付")
	b.consume("A", sword.Name, "B-S")
	b.must(b.b.Accept("q2", "q", "A", sword.Name), "real consumption restores demand")
	b.snap("B3 NPC实际消耗后q可接新订单")
	mintBefore, poolBefore := b.b.Mint(), b.b.Pool("A")
	b.must(b.b.Cancel("q2"), "cancel")
	if b.b.Mint() != mintBefore || b.b.Pool("A") != poolBefore {
		t.Fatal("cancel minted or refunded")
	}
	b.snap("B4 取消仅解冻200")
	b.moveMoney("p", "q", "A", 30)
	b.snap("B5 支付玩家工匠30为转移，非回收")
	compare(t, "B", b.logs, golden["B"])

	// Negative fixtures.
	f := newWorld(t, d, map[string]int64{"p": 0})
	f.b.SetDemand("A", sword.Name, 2)
	f.b.SetDemand("B", sword.Name, 2)
	f.must(f.b.Accept("alt1", "p", "A", sword.Name), "quota 200")
	f.must(f.b.Accept("alt2", "p", "B", sword.Name), "quota 400")
	is(t, f.b.Accept("alt3", "p", "B", sword.Name), ErrAccountLimit, "cross-node quota")
	f.must(f.b.Cancel("alt2"), "cancel")
	f.must(f.b.Accept("alt3", "p", "B", sword.Name), "cancel frees quota")
	f.b.Refill(24)
	is(t, f.b.Accept("alt4", "p", "B", sword.Name), ErrAccountLimit, "open commitments do not expire")
	f.snap("负例quota：未交付预占不因24h自动消失")

	g := newWorld(t, d, map[string]int64{"p": 0, "q": 0})
	g.b.pools["A"] = 100
	g.initial = g.total()
	g.b.SetDemand("A", sword.Name, 1)
	is(t, g.b.Accept("budget", "q", "A", sword.Name), ErrNoBudget, "budget")
	g.snap("负例资金不足：不接单不冻结")

	h := newWorld(t, d, map[string]int64{"p": 0})
	h.b.pools["A"] = 2900
	h.initial = h.total()
	h.b.SetDemand("A", sword.Name, 1)
	h.must(h.b.Accept("frozen", "p", "A", sword.Name), "frozen")
	h.b.Refill(1)
	if h.b.Pool("A") != 3000 || h.b.Frozen("A") != 200 {
		t.Fatal("frozen must count toward the cap")
	}
	h.snap("负例含冻结上限：A仅补100而非200")
	compare(t, "negative_fixtures", [][]any{f.logs, g.logs, h.logs}, golden["negative_fixtures"])

	k := newWorld(t, d, map[string]int64{"p": 0})
	k.b.slots[slotKey{"p", "A"}] = 200
	k.b.SetDemand("A", sword.Name, 1)
	is(t, k.b.Accept("full", "p", "A", sword.Name), ErrNoCapacity, "capacity preflight")

	// C: nobody online for 24h, then 8h maintenance without catch-up.
	z := newWorld(t, d, map[string]int64{"offline": 0})
	z.snap("C0 无人在线且无合同")
	for hour := 1; hour <= 24; hour++ {
		z.b.Refill(1)
		z.snap("C" + strconv.Itoa(hour))
	}
	if z.b.Mint() != 4000 || z.b.NPCPaid() != 0 || z.total() != 6000 {
		t.Fatal("24h capped refill")
	}
	z.b.Maintenance(8)
	if z.total() != 6000 {
		t.Fatal("downtime minted")
	}
	z.snap("C维护8小时不补池")
	compare(t, "C", z.logs, golden["C"])

	// D: physical goods already at the acceptance station; owner offline.
	o := newWorld(t, d, map[string]int64{"owner": 100})
	o.b.SetDemand("A", sword.Name, 1)
	o.buy("owner", "A", sword.Manufacture)
	o.craft("owner", "A", sword, "OFFLINE-S")
	o.must(o.b.Accept("off", "owner", "A", sword.Name), "offline order")
	o.snap("D0 离线前实物已在授权验收站，报酬冻结200")
	o.must(o.deliver("off", "OFFLINE-S"), "offline acceptance")
	o.snap("D1 系统验收，无需货主在线，200进入A待领")
	compare(t, "D_offline_existing_contract", o.logs, golden["D_offline_existing_contract"])

	// Rolling window: paid income expires exactly after 24h.
	r, _ := scenarioA()
	if r.b.Quota("solo") != 350 {
		t.Fatal("rolling before")
	}
	r.b.Refill(24)
	if r.b.Quota("solo") != 0 {
		t.Fatal("rolling income must expire after 24h")
	}
	r.snap("独立滚动额度夹具，主场景A不推进")
	compare(t, "rolling_fixture", r.logs[len(r.logs)-1:], golden["rolling_fixture"])
}

func TestNegativeDeliveries(t *testing.T) {
	d, err := gamedata.Load("../../../shared/data")
	if err != nil {
		t.Fatal(err)
	}
	sword := w0spec(t, d, "测试铁剑")
	w := newWorld(t, d, map[string]int64{"p": 1000, "q": 1000})
	w.b.SetDemand("A", sword.Name, 2)
	w.buy("p", "A", maps.Clone(sword.Manufacture))
	w.buy("q", "A", maps.Clone(sword.Manufacture))
	w.craft("p", "A", sword, "P-S")
	w.craft("q", "A", sword, "Q-S")
	w.must(w.b.Accept("o", "p", "A", sword.Name), "accept")
	is(t, w.deliver("o", "Q-S"), ErrInvalidItem, "someone else's item")
	is(t, w.deliver("o", "MISSING"), ErrInvalidItem, "missing item")
	w.must(w.goods["P-S"].eq.Wear(1), "wear")
	is(t, w.deliver("o", "P-S"), ErrInvalidItem, "worn item")
	is(t, w.deliver("nope", "Q-S"), ErrNotOpen, "unknown order")
	is(t, w.b.Accept("o", "q", "A", sword.Name), ErrDuplicateOrder, "duplicate id")
	is(t, w.b.Consume("A", sword.Name, []string{"Q-S"}), ErrNotInStock, "consume player goods")
	if w.b.NPCPaid() != 0 || w.b.Order("o").State != Open {
		t.Fatal("rejected deliveries changed the book")
	}
}

func w0spec(t *testing.T, d *gamedata.Data, name string) gamedata.Equipment {
	t.Helper()
	eq := d.EquipmentByName(name)
	if eq == nil {
		t.Fatalf("no equipment %q", name)
	}
	return *eq
}

func ledgerInitial(t *testing.T, id string) map[string]int64 {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/design/v0.44/equipment_ledger_v41.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Items []struct {
			ID               string           `json:"id"`
			InitialInventory map[string]int64 `json:"initial_inventory"`
		}
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, it := range f.Items {
		if it.ID == id {
			return it.InitialInventory
		}
	}
	t.Fatalf("no ledger item %s", id)
	return nil
}
