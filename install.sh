#!/usr/bin/env bash
set -euo pipefail

REPO="arnavsurve/streamsh"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# If run from inside the repo, build from source
if [ -f "go.mod" ] && grep -q "arnavsurve/streamsh" go.mod 2>/dev/null; then
  if ! command -v go &>/dev/null; then
    echo "error: go is not installed" >&2
    exit 1
  fi

  echo "Building from source..."
  go install ./cmd/streamsh
  go install ./cmd/streamshd

  GOBIN="$(go env GOPATH)/bin"
  echo "Installed to $GOBIN"

  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$GOBIN"; then
    echo ""
    echo "Warning: $GOBIN is not in your \$PATH."
    echo "Add this to your shell profile:"
    echo ""
    echo "  export PATH=\"\$PATH:$GOBIN\""
  fi
  exit 0
fi

# Otherwise, download prebuilt binaries from GitHub Releases
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

echo "Detecting platform: ${OS}/${ARCH}"

# Get latest release tag
if command -v curl &>/dev/null; then
  LATEST="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
elif command -v wget &>/dev/null; then
  LATEST="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
else
  echo "error: curl or wget required" >&2
  exit 1
fi

if [ -z "$LATEST" ]; then
  echo "error: could not determine latest release" >&2
  exit 1
fi

VERSION="${LATEST#v}"
ARCHIVE="streamsh_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ARCHIVE}"

echo "Downloading ${ARCHIVE}..."

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

if command -v curl &>/dev/null; then
  curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"
else
  wget -q "$URL" -O "$TMPDIR/$ARCHIVE"
fi

tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
cp "$TMPDIR/streamsh" "$INSTALL_DIR/streamsh"
cp "$TMPDIR/streamshd" "$INSTALL_DIR/streamshd"
chmod +x "$INSTALL_DIR/streamsh" "$INSTALL_DIR/streamshd"

# On macOS, downloaded binaries get a persistent com.apple.provenance attribute
# that causes Gatekeeper to SIGKILL ad-hoc signed processes. Re-signing locally
# replaces the linker signature and clears the provenance flag.
if [ "$OS" = "darwin" ] && command -v codesign &>/dev/null; then
  codesign --force --sign - "$INSTALL_DIR/streamsh" 2>/dev/null || true
  codesign --force --sign - "$INSTALL_DIR/streamshd" 2>/dev/null || true
fi

echo "Installed to $INSTALL_DIR"

if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  echo ""
  echo "Warning: $INSTALL_DIR is not in your \$PATH."
  echo "Add this to your shell profile:"
  echo ""
  echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
fi

echo ""
echo "Verify with:"
echo "  streamsh --help"
echo "  streamshd --help"
