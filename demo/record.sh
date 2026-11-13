#!/usr/bin/env bash
# Record the lazytempo demo GIF into assets/demo.gif.
#
# Builds the binary and a throwaway mock Jira+Tempo API (fictional data only),
# then drives the real TUI with VHS. No real Jira/Tempo account is used.
#
# Requires: go, and vhs (https://github.com/charmbracelet/vhs — `brew install vhs`).
set -euo pipefail

cd "$(dirname "$0")/.."  # repo root

command -v vhs >/dev/null 2>&1 || {
	echo "error: vhs not found. Install it: brew install vhs" >&2
	exit 1
}

echo "==> building binary"
go build -o ./lazytempo ./cmd/lazytempo

echo "==> building mock API"
MOCK_BIN="$(mktemp -t lt-mock.XXXXXX)"
go build -o "$MOCK_BIN" demo/mock.go

echo "==> starting mock on :8787"
"$MOCK_BIN" &
MOCK_PID=$!
cleanup() { kill "$MOCK_PID" 2>/dev/null || true; rm -f "$MOCK_BIN"; }
trap cleanup EXIT

# Wait for the mock to accept connections (max ~5s).
for _ in $(seq 1 50); do
	if curl -sf -o /dev/null http://localhost:8787/rest/api/3/myself; then break; fi
	sleep 0.1
done

echo "==> recording (vhs demo/demo.tape)"
vhs demo/demo.tape

echo "==> wrote assets/demo.gif"
