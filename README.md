# Jianghu · 江湖

English | [简体中文](README.zh-CN.md)

An original wuxia multiplayer online growth game: real-time combat, martial-arts lineages, sects and life professions, set in one persistent world that a single player can keep playing on their own.

**Current stage: design plus engineering skeleton, with the first rule modules in place. This is not a playable multiplayer game yet.**

- Client: Godot 4 with typed GDScript.
- Server: Go.
- Delivery targets: Windows, macOS, iOS and Android, all four required. No platform has passed release acceptance yet.
- The server is authoritative for all results. Persistent assets are stored in PostgreSQL.

## Repository layout

```text
client/              Godot project, boot scene, scripts and client tests
server/              Go server: config, HTTP diagnostics, rule packages, unit tests
shared/data/         Versioned test data loaded by the server (schema_version 1)
shared/protocol/     Client/server protocol notes
assets/concept-art/  Scene and character concept art with generation prompts
docs/                Design snapshot, decisions, data dictionary, verification records
scripts/             Local check and data export scripts
.github/workflows/   CI checks
```

## Quick start

Requires Go 1.25+, Python 3 and the Godot 4 standard build. Third-party Go dependencies: `pgx` (PostgreSQL), `coder/websocket` and `golang.org/x/crypto`. A database is optional: without `JIANGHU_DATABASE_URL` the server runs diagnostics only (see [server/README.md](server/README.md)).

```sh
# from the repository root
cd server
go run ./cmd/jianghu-server
```

In another terminal:

```sh
curl http://127.0.0.1:8080/healthz
# returns JSON with status=ok; this only proves the HTTP process is up
```

Import `client/project.godot` in Godot, run the main scene and press the health-check button. When testing on a phone, start the server on an explicit LAN address and enter your computer's LAN IP in the client; `127.0.0.1` on a phone means the phone itself. Do not expose this unauthenticated skeleton to the public internet.

```sh
# Go tests, data export check, import checksums and design model checks
./scripts/check.sh
# Godot headless tests: see client/README.md
```

## What exists today

| Area | State |
|---|---|
| Server transport | HTTP `/healthz` and `/version`; with a database, WebSocket `/v1/ws` for accounts, characters, entering/leaving the world, movement and snapshots ([protocol v1](shared/protocol/v1.md)) |
| Test data | `shared/data/`: items, recipes, economy, moves, teaching routes, map and timing parameters. Four files are generated from the design snapshot by `scripts/export_data.py`; CI fails if they drift |
| Server rule packages | `gamedata` (load and cross-check data), `growth` (attribute settlement in 0.1 steps), `crafting` (manufacture, wear, repair, salvage), `economy` (NPC limited-purchase orders), `ledger` (stack, slot and carry-weight checks) |
| Asset ledger | `store`: PostgreSQL migrations, idempotent requests (a replayed request ID returns the first result), unique item locations, non-negative coins, ordered container locks |
| Evidence | The Go rules reproduce the design's Python calculations: 18/18 growth cases, the full equipment material ledger, and every economy scenario snapshot field by field. Under 50 concurrent requests an item is picked up exactly once and coins never go negative. A second login kicks the first connection and only one character per account is in the world. See [M0](docs/verification/m0-data-and-rules.md), [M1](docs/verification/m1-persistence.md), [M2](docs/verification/m2-accounts-protocol.md) and [M3](docs/verification/m3-world-movement.md) verification |
| Client | Boot page, account screen, and a six-block graybox world: right-click (or tap on mobile) to move, other players visible, risk-area confirmation, zoom/rotate camera |
| World | `world`: one authoritative 20 Hz loop; the server plans paths and walks at 4 m/s, clients only send destinations; ordinary routes avoid risk areas; positions are saved on leave |

Not implemented yet: combat, items in play, trading, transport contracts, TLS and deployment. Builds: macOS exported and launched locally; Windows and Android are exported by CI; iOS needs an Apple team ID. All design numbers are test parameters, and all 16 playable acceptance checks are still pending.

## Development plan

Work follows the stage gates in the design baseline. Each stage closes only with test evidence.

1. **M0 Data and rules**: done.
2. **M1 Persistence and transactions**: done.
3. **M2 Accounts and real-time protocol**: done.
4. **M3 World and movement**: done.
5. **M4–M7 Gameplay**: combat and death, two-node storage and logistics, life skills and equipment, quests and lineages.
6. **M8 First playable acceptance (G1)**: all 16 checks executed with recorded evidence.

## Docs and art

- [Documentation index](docs/README.md) (Chinese)
- [Technical baseline: Godot + Go](docs/decisions/0001-technical-baseline.md)
- [Design/technical baseline reconciliation](docs/decisions/0002-baseline-reconciliation.md)
- [Data field dictionary](docs/data/field-dictionary.md) and [state machines](docs/data/state-machines.md)
- [Design snapshot](docs/design/v0.44/游戏策划设定.md)
- [Concept art](assets/concept-art/README.md)

The design and decision documents are written in Chinese.

![Starter village overview](assets/concept-art/starter-village-wide-v1.png)

## License and contributing

Released under the [MIT License](LICENSE). Third-party dependencies and assets keep their own licenses. Never commit secrets, real player data, signing certificates or assets of unknown origin. The source and rights boundaries of the AI concept art are described in the concept art notes. Run the local checks before committing, and update tests and related docs whenever you change a rule.
