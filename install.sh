#!/usr/bin/env bash
# install.sh - Install glpictl-ai on Linux/macOS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.sh | bash
#   # or
#   wget -qO- https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.sh | bash

set -euo pipefail

REPO="giulianotesta7/glpictl-ai"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="glpictl-ai"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Detect OS
detect_os() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$OS" in
        linux|darwin) ;;
        *)
            error "Unsupported OS: $OS"
            exit 1
            ;;
    esac
    echo "$OS"
}

# Detect architecture
detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)
            error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    echo "$ARCH"
}

# Get latest release version
get_latest_version() {
    info "Fetching latest release information..."
    LATEST_RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)

    if [[ -z "$LATEST_RELEASE" ]]; then
        error "Failed to fetch release information from GitHub"
        exit 1
    fi

    VERSION=$(echo "$LATEST_RELEASE" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)

    if [[ -z "$VERSION" ]]; then
        error "Could not determine latest version"
        exit 1
    fi

    echo "$VERSION"
}

# Download binary
download_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local binary_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${version}-${os}-${arch}"

    info "Downloading ${BINARY_NAME} ${version} for ${os}/${arch}..."

    # Try the versioned binary name first, fall back to simple name
    if ! curl -fsSL --fail "${binary_url}" -o "/tmp/${BINARY_NAME}" 2>/dev/null; then
        binary_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${os}-${arch}"
        if ! curl -fsSL --fail "${binary_url}" -o "/tmp/${BINARY_NAME}" 2>/dev/null; then
            error "Failed to download binary from GitHub releases"
            error "Tried URLs:"
            error "  ${binary_url}"
            exit 1
        fi
    fi
}

# Install binary
install_binary() {
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."

    mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    info "Installation complete!"
}

# Run configure
run_configure() {
    info "Running configuration..."
    echo ""
    "${INSTALL_DIR}/${BINARY_NAME}" configure
}

# Main
main() {
    echo "========================================="
    echo "  glpictl-ai Installer"
    echo "========================================="
    echo ""

    check_root

    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detected: ${OS}/${ARCH}"

    VERSION=$(get_latest_version)
    info "Latest version: ${VERSION}"

    download_binary "$VERSION" "$OS" "$ARCH"
    install_binary

    echo ""
    run_configure
}

main "$@"
