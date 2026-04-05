#!/usr/bin/env bash
# install.sh - Install glpictl-ai on Linux/macOS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.sh | bash
#   # or
#   wget -qO- https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.sh | bash

set -euo pipefail

REPO="giulianotesta7/glpictl-ai"
INSTALL_DIR="${HOME}/.local/bin"
SYSTEM_MODE=false
BINARY_NAME="glpictl-ai"

# Global temp files — trap set once at top-level scope so it covers ALL exit paths
DOWNLOAD_FILE=""
CHECKSUM_FILE=""
trap 'rm -f "$DOWNLOAD_FILE" "$CHECKSUM_FILE"' EXIT

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

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --dir)
                if [[ -z "${2:-}" ]]; then
                    error "--dir requires a path argument"
                    exit 1
                fi
                INSTALL_DIR="$2"
                shift 2
                ;;
            --system)
                SYSTEM_MODE=true
                INSTALL_DIR="/usr/local/bin"
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                error "Unknown argument: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Show usage information
usage() {
    echo "Usage: ./install.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dir <path>    Install to custom directory (default: ~/.local/bin)"
    echo "  --system        Install to /usr/local/bin (requires sudo)"
    echo "  -h, --help      Show this help message"
}

# Ensure install directory exists and is writable
ensure_install_dir() {
    if [[ ! -d "$INSTALL_DIR" ]]; then
        if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
            error "Cannot create ${INSTALL_DIR}"
            error "Try: sudo ./install.sh --system"
            exit 1
        fi
    fi
    
    # Check writability
    if [[ ! -w "$INSTALL_DIR" ]]; then
        error "Cannot write to ${INSTALL_DIR}"
        error "Try: sudo ./install.sh --system"
        exit 1
    fi
}

# Check if install directory is in PATH, warn if not
check_path() {
    # Skip for system-wide install — /usr/local/bin is always in PATH
    [[ "$SYSTEM_MODE" == true ]] && return 0
    
    # Check if INSTALL_DIR is in PATH
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo "  Add it with: echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.bashrc"
        echo "  Or add this line to your shell profile:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
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
    # NOTE: Do NOT call info() here - its output would be captured in VERSION
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

    # Create a private temp file to avoid TOCTOU race in world-writable /tmp
    DOWNLOAD_FILE=$(mktemp "/tmp/${BINARY_NAME}.XXXXXX")

    # Try the versioned binary name first, fall back to simple name
    local primary_url="$binary_url"
    if ! curl -fsSL --fail "${binary_url}" -o "$DOWNLOAD_FILE" 2>/dev/null; then
        binary_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${os}-${arch}"
        if ! curl -fsSL --fail "${binary_url}" -o "$DOWNLOAD_FILE" 2>/dev/null; then
            error "Failed to download binary from GitHub releases"
            error "Tried URLs:"
            error "  ${primary_url}"
            error "  ${binary_url}"
            exit 1
        fi
    fi

    # Verify checksum if available (graceful: warn but continue if missing)
    verify_checksum "$DOWNLOAD_FILE" "$version"
}

# Install binary
install_binary() {
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."

    mv "$DOWNLOAD_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    info "Installation complete!"
}

# Verify checksum if available (graceful: warn but continue if missing)
verify_checksum() {
    local file="$1"
    local version="$2"
    local checksum_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${version}-${OS}-${ARCH}.sha256"

    # Use mktemp for the checksum file to avoid TOCTOU race in /tmp
    CHECKSUM_FILE=$(mktemp "/tmp/${BINARY_NAME}.checksum.XXXXXX")

    # Fall back to simple name for checksum file
    if ! curl -fsSL --fail "${checksum_url}" -o "$CHECKSUM_FILE" 2>/dev/null; then
        checksum_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${OS}-${ARCH}.sha256"
        if ! curl -fsSL --fail "${checksum_url}" -o "$CHECKSUM_FILE" 2>/dev/null; then
            warn "No checksum file found at release — skipping verification"
            warn "For full supply-chain security, publish .sha256 checksums with releases"
            rm -f "$CHECKSUM_FILE"
            CHECKSUM_FILE=""
            return 0
        fi
    fi

    info "Verifying checksum..."
    expected_hash=$(grep -F -m1 "${BINARY_NAME}-${version}-${OS}-${ARCH}" "$CHECKSUM_FILE" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
    if [[ -z "$expected_hash" ]]; then
        # Fall back to simple binary name if versioned name not found
        expected_hash=$(grep -F -m1 "${BINARY_NAME}-${OS}-${ARCH}" "$CHECKSUM_FILE" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
    fi

    # Use OS-appropriate hash command
    if [[ "$OS" == "linux" ]]; then
        actual_hash=$(sha256sum "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
    else
        actual_hash=$(shasum -a 256 "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
    fi

    rm -f "$CHECKSUM_FILE"
    CHECKSUM_FILE=""
    if [[ "$expected_hash" != "$actual_hash" ]]; then
        error "Checksum verification FAILED — binary may be tampered with"
        exit 1
    fi
    info "Checksum verification passed!"
}

# Run configure
run_configure() {
    info "Running configuration..."
    echo ""
    # If running under sudo, run configure as the original user so config
    # is saved to the user's ~/.config/ instead of /root/.config/
    if [[ -n "${SUDO_USER:-}" ]]; then
        sudo -u "$SUDO_USER" "${INSTALL_DIR}/${BINARY_NAME}" configure
    else
        "${INSTALL_DIR}/${BINARY_NAME}" configure
    fi
}

# Run MCP client setup
run_setup_mcp() {
    echo ""
    info "Setting up MCP clients..."
    echo ""
    # If running under sudo, run as the original user so configs
    # are saved to the user's home directory
    if [[ -n "${SUDO_USER:-}" ]]; then
        sudo -u "$SUDO_USER" "${INSTALL_DIR}/${BINARY_NAME}" setup-mcp
    else
        "${INSTALL_DIR}/${BINARY_NAME}" setup-mcp
    fi
}

# Main
main() {
    echo "========================================="
    echo "  glpictl-ai Installer"
    echo "========================================="
    echo ""

    # Parse command line arguments first
    parse_args "$@"

    OS=$(detect_os)
    ARCH=$(detect_arch)

    # Global temp files are declared and trapped at top-level scope.
    DOWNLOAD_FILE=""
    CHECKSUM_FILE=""

    info "Detected: ${OS}/${ARCH}"

    VERSION=$(get_latest_version)
    info "Latest version: ${VERSION}"

    download_binary "$VERSION" "$OS" "$ARCH"
    
    # Ensure install directory exists and is writable
    ensure_install_dir
    
    install_binary

    # Check if install directory is in PATH
    check_path

    info "Installation complete!"
    info "Run 'glpictl-ai configure' to set up your GLPI connection"
}

main "$@"
