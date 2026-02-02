#!/usr/bin/env bash
set -euo pipefail

if ! command -v go &>/dev/null; then
  echo "error: go is not installed" >&2
  exit 1
fi

echo "Building streamsh and streamshd..."
go install ./cmd/streamsh
go install ./cmd/streamshd

GOBIN="$(go env GOPATH)/bin"

echo "Installed to $GOBIN"
echo ""

if ! echo "$PATH" | tr ':' '\n' | grep -qx "$GOBIN"; then
  echo "Warning: $GOBIN is not in your \$PATH."
  echo "Add this to your shell profile:"
  echo ""
  echo "  export PATH=\"\$PATH:$GOBIN\""
  echo ""
fi

echo "Verify with:"
echo "  streamsh --help"
echo "  streamshd --help"
