#!/usr/bin/env bash
set -euo pipefail

REPO="tornikegomareli/claudeignore"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="claudeignore"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() { echo -e "${CYAN}[installer]${NC} $*"; }
ok()  { echo -e "${GREEN}[installer]${NC} $*"; }
err() { echo -e "${RED}[installer]${NC} $*" >&2; exit 1; }

# Detect platform and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    *)      err "Unsupported OS: $OS. Only macOS and Linux are supported." ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)             err "Unsupported architecture: $ARCH" ;;
esac

log "Installing claudeignore for ${OS}/${ARCH}..."

# Create install directory
mkdir -p "$INSTALL_DIR"

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed 's/.*"tag_name":[[:space:]]*"//;s/".*//')

if [[ -z "$LATEST" ]]; then
    # No releases yet — build from source
    log "No release found. Building from source..."

    if ! command -v go &>/dev/null; then
        err "Go is required to build from source. Install it from https://go.dev"
    fi

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    git clone --depth 1 "https://github.com/${REPO}.git" "$TMPDIR/src" 2>/dev/null || \
        err "Failed to clone repository"

    (cd "$TMPDIR/src" && go build -o "${INSTALL_DIR}/${BINARY_NAME}" .) || \
        err "Build failed"
else
    # Download prebuilt binary from release
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/claudeignore-${OS}-${ARCH}"
    log "Downloading ${LATEST} from GitHub releases..."

    if command -v curl &>/dev/null; then
        curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/${BINARY_NAME}" || \
            err "Download failed. URL: $DOWNLOAD_URL"
    elif command -v wget &>/dev/null; then
        wget -q "$DOWNLOAD_URL" -O "${INSTALL_DIR}/${BINARY_NAME}" || \
            err "Download failed. URL: $DOWNLOAD_URL"
    else
        err "Neither curl nor wget found."
    fi
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Verify binary works
if ! "${INSTALL_DIR}/${BINARY_NAME}" version &>/dev/null; then
    err "Binary verification failed. Try building from source:\n  go install github.com/${REPO}@latest"
fi

# Check if INSTALL_DIR is in PATH
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    SHELL_NAME=$(basename "$SHELL")
    case "$SHELL_NAME" in
        zsh)  RC_FILE="$HOME/.zshrc" ;;
        bash) RC_FILE="$HOME/.bashrc" ;;
        *)    RC_FILE="$HOME/.profile" ;;
    esac

    if [[ -t 0 ]]; then
        log "${INSTALL_DIR} is not in your PATH."
        read -rp "Add to $RC_FILE? [y/N] " answer
        if [[ "$answer" =~ ^[Yy] ]]; then
            echo 'export PATH="'"${INSTALL_DIR}"':$PATH"' >> "$RC_FILE"
            ok "Added to $RC_FILE — run: source $RC_FILE"
        else
            log "Add manually:  export PATH=\"${INSTALL_DIR}:\$PATH\""
        fi
    else
        log "Add to your PATH:  export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
fi

# Register hook globally — this is the whole point
log "Registering hook in ~/.claude/settings.json..."
"${INSTALL_DIR}/${BINARY_NAME}" setup

echo ""
ok "Installed claudeignore v$(${INSTALL_DIR}/${BINARY_NAME} version | awk '{print $2}')"
echo ""
log "Now just drop a .claudeignore file in any project:"
log "  cd your-project/"
log "  echo 'node_modules/' > .claudeignore"
log ""
log "That's it. No other steps needed."
echo ""
