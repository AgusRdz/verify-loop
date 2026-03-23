#!/bin/sh
set -e

REPO="agusrdz/verify-loop"

OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) echo "unsupported OS: $OS" >&2; exit 1 ;;
esac

if [ -z "$VERIFY_LOOP_INSTALL_DIR" ]; then
  if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$(cygpath "$LOCALAPPDATA/Programs/verify-loop" 2>/dev/null || echo "$HOME/AppData/Local/Programs/verify-loop")"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
else
  INSTALL_DIR="$VERIFY_LOOP_INSTALL_DIR"
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

EXT=""
if [ "$OS" = "windows" ]; then EXT=".exe"; fi

BINARY="verify-loop_${OS}_${ARCH}${EXT}"

if [ -z "$VERIFY_LOOP_VERSION" ]; then
  VERIFY_LOOP_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"//;s/".*//')
fi

if [ -z "$VERIFY_LOOP_VERSION" ]; then
  echo "failed to determine latest version" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${VERIFY_LOOP_VERSION}/${BINARY}"

echo "Installing verify-loop ${VERIFY_LOOP_VERSION}..."
mkdir -p "$INSTALL_DIR"

TMP="$(mktemp)"
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"
mv "$TMP" "${INSTALL_DIR}/verify-loop${EXT}"

echo ""
echo "Installed: ${INSTALL_DIR}/verify-loop${EXT}"
echo ""
echo "Next steps:"
echo "  1. Add ${INSTALL_DIR} to your PATH if not already there"
echo "  2. Run: verify-loop init"
echo "  3. That's it — checks run automatically on every Claude Write"
echo ""
echo "Quick start:"
echo "  verify-loop run src/app.ts     # manually check a file"
echo "  verify-loop doctor             # diagnose any issues"
echo "  verify-loop disable            # temporarily silence checks"
