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

# Register in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    if [ "$OS" = "windows" ]; then
      # Update Windows registry PATH for native tools
      WIN_DIR=$(cygpath -w "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")
      powershell.exe -NoProfile -Command "\$p = [Environment]::GetEnvironmentVariable('Path', 'User'); \$d = '${WIN_DIR}'.TrimEnd('\\'); if ((\$p -split ';' | ForEach-Object { \$_.TrimEnd('\\') }) -notcontains \$d) { [Environment]::SetEnvironmentVariable('Path', \"\$d;\$p\", 'User'); Write-Host \"Added \$d to User PATH (registry)\" }" 2>/dev/null || true
      # Also write to .bashrc for Git Bash / MSYS sessions
      SHELL_RC="$HOME/.bashrc"
      PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""
      if ! grep -qF "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
        printf '\n# verify-loop\n%s\n' "$PATH_LINE" >> "$SHELL_RC"
        echo "Added ${INSTALL_DIR} to PATH in $SHELL_RC"
      fi
      echo "Reload your shell or run: source ~/.bashrc"
    else
      SHELL_NAME="$(basename "${SHELL:-}")"
      case "$SHELL_NAME" in
        zsh)  SHELL_RC="$HOME/.zshrc" ;;
        bash) SHELL_RC="$HOME/.bashrc" ;;
        *)    SHELL_RC="" ;;
      esac
      PATH_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""
      if [ -n "$SHELL_RC" ]; then
        if ! grep -qF "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
          printf '\n# verify-loop\n%s\n' "$PATH_LINE" >> "$SHELL_RC"
          echo "Added ${INSTALL_DIR} to PATH in $SHELL_RC"
          echo "Reload your shell with: source $SHELL_RC"
        fi
      else
        echo "NOTE: add ${INSTALL_DIR} to your PATH:"
        echo "  $PATH_LINE"
      fi
    fi
    ;;
esac

echo ""
echo "Next steps:"
echo "  1. Run: verify-loop init"
echo "  2. That's it — checks run automatically on every Claude Write"
echo ""
echo "Quick start:"
echo "  verify-loop run src/app.ts     # manually check a file"
echo "  verify-loop doctor             # diagnose any issues"
echo "  verify-loop disable            # temporarily silence checks"
