#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT/server"
go test -race ./...
go vet ./...
cd "$ROOT"
python3 scripts/check_imports.py
python3 scripts/export_data.py --check
python3 docs/design/v0.44/verify_growth_v37.py
python3 docs/design/v0.44/verify_routes_v39.py
python3 docs/design/v0.44/verify_equipment_ledger_v41.py
OUT=$(mktemp -d)
trap 'rm -rf "$OUT"' EXIT HUP INT TERM
python3 docs/design/v0.44/verify_economy_v44.py --input docs/design/v0.44/economy_v44.json --output-prefix "$OUT/economy"
