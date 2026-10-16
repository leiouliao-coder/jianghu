#!/bin/sh
# Runs Godot headless test scripts. A test fails if Godot exits non-zero,
# prints a SCRIPT ERROR (a GDScript error can abort a test without changing
# the exit code), or never prints its PASS line.
# Usage: scripts/run_client_tests.sh <godot-binary> <test-name>... [-- <user args>]
set -eu
GODOT=$1
shift
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TESTS=""
while [ $# -gt 0 ] && [ "$1" != "--" ]; do
	TESTS="$TESTS $1"
	shift
done
[ "${1:-}" = "--" ] && shift
OUT=$(mktemp)
trap 'rm -f "$OUT"' EXIT HUP INT TERM
failed=0
for t in $TESTS; do
	status=0
	"$GODOT" --headless --path "$ROOT/client" --script "res://tests/$t.gd" -- "$@" >"$OUT" 2>&1 || status=$?
	if [ "$status" -ne 0 ] || grep -q 'SCRIPT ERROR' "$OUT" || ! grep -q '^PASS' "$OUT"; then
		echo "FAIL $t (exit $status)"
		grep -E 'SCRIPT ERROR|ERROR|FAIL|at: ' "$OUT" | head -20
		failed=1
	else
		grep '^PASS' "$OUT"
	fi
done
exit $failed
