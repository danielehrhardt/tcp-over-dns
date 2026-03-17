#!/usr/bin/env bash
#
# tcpdns installer script
# Detects OS/arch and downloads the latest release binary
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/danielehrhardt/tcp-over-dns/main/scripts/install.sh | bash
#

set -euo pipefail

REPO="danielehrhardt/tcp-over-dns"
BINARY="tcpdns"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}[*]${NC} $*"; }
success() { echo -e "${GREEN}[+]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
error()   { echo -e "${RED}[-]${NC} $*"; exit 1; }

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)      error "Unsupported OS: $OS. For Windows, download from GitHub Releases." ;;
    esac

    info "Detected platform: ${OS}/${ARCH}"
}

get_latest_version() {
    info "Fetching latest release..."
    local response
    response=$(curl -sSL -w "\n%{http_code}" "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null) || true

    local http_code
    http_code=$(echo "$response" | tail -1)
    local body
    body=$(echo "$response" | sed '$d')

    if [[ "$http_code" != "200" ]]; then
        error "No releases found (HTTP ${http_code}). Check https://github.com/${REPO}/releases\n\n  To install from source instead:\n    go install github.com/${REPO}/cmd/tcpdns@latest"
    fi

    VERSION=$(echo "$body" | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/') || true
    if [[ -z "$VERSION" ]]; then
        error "Could not parse version from release. Check https://github.com/${REPO}/releases"
    fi
    info "Latest version: v${VERSION}"
}

download_binary() {
    ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

    info "Downloading ${URL}..."

    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT

    if command -v curl &>/dev/null; then
        curl -sSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
    elif command -v wget &>/dev/null; then
        wget -q "$URL" -O "${TMP_DIR}/${ARCHIVE}"
    else
        error "Neither curl nor wget found. Please install one."
    fi

    info "Extracting..."
    tar xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

    if [[ ! -f "${TMP_DIR}/${BINARY}" ]]; then
        error "Binary not found in archive"
    fi
}

install_binary() {
    info "Installing to ${INSTALL_DIR}/${BINARY}..."

    if [[ -w "$INSTALL_DIR" ]]; then
        mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        warn "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY}"
    success "Installed ${BINARY} v${VERSION} to ${INSTALL_DIR}/${BINARY}"
}

verify_installation() {
    if command -v "$BINARY" &>/dev/null; then
        success "Installation verified: $(${BINARY} version 2>&1 | head -1 || echo 'ok')"
    else
        warn "Binary installed but not in PATH. Add ${INSTALL_DIR} to your PATH."
    fi
}

main() {
    echo ""
    echo -e "${BLUE}tcpdns installer${NC}"
    echo ""

    detect_platform
    get_latest_version
    download_binary
    install_binary
    verify_installation

    echo ""
    success "Done! Get started:"
    echo "  tcpdns config init          # Set up configuration"
    echo "  tcpdns --help               # See all commands"
    echo ""
}

main "$@"
