#!/bin/sh
# Copyright 2025 KrakLabs
# SPDX-License-Identifier: AGPL-3.0-or-later

# CIE Install Script
# Installs the CIE (Code Intelligence Engine) CLI binary
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/kraklabs/cie/main/install.sh | sh
#
# Environment variables:
#   CIE_INSTALL_DIR - Installation directory (default: /usr/local/bin or ~/.local/bin)
#   CIE_VERSION     - Specific version to install (default: latest)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub repository
REPO="kraklabs/cie"
BINARY_NAME="cie"

info() {
    printf "${BLUE}info${NC}: %s\n" "$1"
}

success() {
    printf "${GREEN}success${NC}: %s\n" "$1"
}

warn() {
    printf "${YELLOW}warn${NC}: %s\n" "$1"
}

error() {
    printf "${RED}error${NC}: %s\n" "$1" >&2
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported operating system: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Download file
download() {
    url="$1"
    output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
}

# Verify checksum
verify_checksum() {
    archive="$1"
    checksum_file="$2"

    expected=$(cat "$checksum_file" | awk '{print $1}')

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        warn "Neither sha256sum nor shasum found. Skipping checksum verification."
        return 0
    fi

    if [ "$expected" != "$actual" ]; then
        error "Checksum verification failed!\nExpected: $expected\nActual: $actual"
    fi

    info "Checksum verified"
}

# Determine installation directory
get_install_dir() {
    if [ -n "$CIE_INSTALL_DIR" ]; then
        echo "$CIE_INSTALL_DIR"
    elif [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
    else
        mkdir -p "$HOME/.local/bin"
        echo "$HOME/.local/bin"
    fi
}

# Check if directory is in PATH
check_path() {
    dir="$1"
    case ":$PATH:" in
        *":$dir:"*) return 0 ;;
        *)          return 1 ;;
    esac
}

main() {
    info "CIE Installer"
    echo ""

    # Detect platform
    OS=$(detect_os)
    ARCH=$(detect_arch)
    info "Detected platform: ${OS}/${ARCH}"

    # Get version
    if [ -n "$CIE_VERSION" ]; then
        VERSION="$CIE_VERSION"
    else
        info "Fetching latest version..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Failed to get latest version. Please set CIE_VERSION manually."
        fi
    fi
    info "Installing version: ${VERSION}"

    # Determine install directory
    INSTALL_DIR=$(get_install_dir)
    info "Install directory: ${INSTALL_DIR}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Build download URL
    ARCHIVE_NAME="cie_${VERSION}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
    CHECKSUM_URL="${DOWNLOAD_URL}.sha256"

    # Download archive and checksum
    info "Downloading ${ARCHIVE_NAME}..."
    download "$DOWNLOAD_URL" "$TMP_DIR/$ARCHIVE_NAME"

    info "Downloading checksum..."
    download "$CHECKSUM_URL" "$TMP_DIR/${ARCHIVE_NAME}.sha256"

    # Verify checksum
    cd "$TMP_DIR"
    verify_checksum "$ARCHIVE_NAME" "${ARCHIVE_NAME}.sha256"

    # Extract archive
    info "Extracting archive..."
    tar -xzf "$ARCHIVE_NAME"

    # Find binary (handles cie-darwin-arm64 naming)
    BINARY=$(find . -name "cie-*" -type f | head -1)
    if [ -z "$BINARY" ]; then
        error "Binary not found in archive"
    fi

    # Install binary
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    chmod +x "$BINARY"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo mv "$BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    echo ""
    success "CIE ${VERSION} installed successfully!"
    echo ""

    # Check if install dir is in PATH
    if ! check_path "$INSTALL_DIR"; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "Add it to your shell profile:"
        echo ""
        echo "  # For bash (~/.bashrc or ~/.bash_profile)"
        echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
        echo "  # For zsh (~/.zshrc)"
        echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
    fi

    # Print getting started
    echo "Getting started:"
    echo ""
    echo "  # Start the infrastructure"
    echo "  cie start"
    echo ""
    echo "  # Initialize and index your repository"
    echo "  cie init"
    echo "  cie index"
    echo ""
    echo "  # Check status"
    echo "  cie status"
    echo ""
    echo "For more information, visit: https://github.com/${REPO}"
}

main "$@"
