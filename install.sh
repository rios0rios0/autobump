#!/bin/sh
#
# AutoBump Installation Script
#
# Downloads and installs autobump from GitHub releases.
# Automatically detects your operating system and architecture. On Windows it
# runs from Git Bash, MSYS2 or Cygwin, and needs unzip.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh
#   wget -qO- https://raw.githubusercontent.com/rios0rios0/autobump/main/install.sh | sh
#
# Options:
#   --help              Show this help message
#   --version VERSION   Install specific version (e.g. 1.0.0)
#   --install-dir DIR   Custom installation directory (default: ~/.local/bin)
#   --force             Force reinstallation
#   --dry-run           Show what would be done without installing
#
# Environment variables:
#   AUTOBUMP_INSTALL_DIR   Installation directory (default: ~/.local/bin)
#   AUTOBUMP_VERSION       Specific version to install (default: latest)
#   AUTOBUMP_FORCE         Force installation (true/false, default: false)
#   AUTOBUMP_DRY_RUN       Dry run mode (true/false, default: false)
#

set -e

# Project configuration
REPO_OWNER="rios0rios0"
REPO_NAME="autobump"
BINARY_NAME="autobump"
# The file the binary is installed as, which main() sets per platform.
BINARY_FILE="$BINARY_NAME"

# Defaults
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
INSTALL_DIR="${AUTOBUMP_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
VERSION="${AUTOBUMP_VERSION:-latest}"
FORCE="${AUTOBUMP_FORCE:-false}"
DRY_RUN="${AUTOBUMP_DRY_RUN:-false}"

# GitHub API
GITHUB_API_BASE="https://api.github.com"
GITHUB_RELEASE_BASE="https://github.com"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging
info()    { printf "${BLUE}INFO:${NC} %s\n" "$1"; }
warn()    { printf "${YELLOW}WARN:${NC} %s\n" "$1"; }
error()   { printf "${RED}ERROR:${NC} %s\n" "$1" >&2; }
success() { printf "${GREEN}SUCCESS:${NC} %s\n" "$1"; }

# Help
show_help() {
    cat << EOF
${BINARY_NAME} Installation Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    --help              Show this help message
    --version VERSION   Install specific version (e.g. v1.0.0 or 1.0.0)
    --install-dir DIR   Custom installation directory (default: ~/.local/bin)
    --force             Force reinstallation
    --dry-run           Show what would be done without installing

ENVIRONMENT VARIABLES:
    AUTOBUMP_INSTALL_DIR   Installation directory
    AUTOBUMP_VERSION       Specific version to install (default: latest)
    AUTOBUMP_FORCE         Force installation (true/false)
    AUTOBUMP_DRY_RUN       Dry run mode (true/false)

EXAMPLES:
    $0
    $0 --version 1.0.0
    $0 --install-dir /usr/local/bin
    $0 --dry-run
    $0 --force

EOF
}

# Parse arguments
parse_args() {
    while [ $# -gt 0 ]; do
        case $1 in
            --help|-h)   show_help; exit 0 ;;
            --version)   [ -z "$2" ] && { error "Version argument required"; exit 1; }; VERSION="$2"; shift 2 ;;
            --force)     FORCE="true"; shift ;;
            --dry-run)   DRY_RUN="true"; shift ;;
            --install-dir) [ -z "$2" ] && { error "Install directory argument required"; exit 1; }; INSTALL_DIR="$2"; shift 2 ;;
            *)           error "Unknown option: $1"; error "Use --help to see available options"; exit 1 ;;
        esac
    done
}

# Detect operating system
detect_os() {
    case "$(uname -s)" in
        Linux*)                   echo "linux" ;;
        Darwin*)                  echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*)    echo "windows" ;;
        *)  error "Unsupported operating system: $(uname -s). On Windows, run this script from Git Bash, MSYS2 or Cygwin."; exit 1 ;;
    esac
}

# The file name the binary is installed under: Windows only runs an executable
# through its .exe extension.
binary_file() {
    if [ "$1" = "windows" ]; then
        echo "${BINARY_NAME}.exe"
    else
        echo "${BINARY_NAME}"
    fi
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)    echo "amd64" ;;
        arm64|aarch64)   echo "arm64" ;;
        # Releases are built for amd64 and arm64 only, so a 32-bit x86 or ARM
        # machine would otherwise get a 404 for an asset that never existed.
        *)  error "Unsupported architecture: $(uname -m). Releases are published for amd64 and arm64 only."; exit 1 ;;
    esac
}

# Check if a command exists
command_exists() { command -v "$1" >/dev/null 2>&1; }

# Ensure curl or wget is available
check_download_tool() {
    if command_exists curl; then
        DOWNLOAD_CMD="curl"
    elif command_exists wget; then
        DOWNLOAD_CMD="wget"
    else
        error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
}

# Download a URL to a local file
download_file() {
    url="$1"; output="$2"
    if [ "$DOWNLOAD_CMD" = "curl" ]; then
        curl -fsSL -o "$output" "$url"
    else
        wget -q -O "$output" "$url"
    fi
}

# Resolve the tag name for the latest release
get_latest_tag() {
    api_url="${GITHUB_API_BASE}/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
    tmp=$(mktemp)

    if ! download_file "$api_url" "$tmp"; then
        rm -f "$tmp"
        error "Failed to fetch latest release from GitHub API"
        exit 1
    fi

    tag=$(grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' "$tmp" | cut -d'"' -f4)
    rm -f "$tmp"

    if [ -z "$tag" ]; then
        error "Could not parse release tag from GitHub API response"
        exit 1
    fi
    echo "$tag"
}

# Build the download URL for a given version, OS, and architecture.
# GoReleaser naming: {project}-{version}-{os}-{arch}.tar.gz (.zip on Windows)
build_download_url() {
    tag="$1"; os="$2"; arch="$3"
    ver=$(echo "$tag" | sed 's/^v//')

    ext="tar.gz"
    [ "$os" = "windows" ] && ext="zip"

    echo "${GITHUB_RELEASE_BASE}/${REPO_OWNER}/${REPO_NAME}/releases/download/${tag}/${BINARY_NAME}-${ver}-${os}-${arch}.${ext}"
}

# Check if the binary is already installed
check_existing_installation() {
    if [ -f "${INSTALL_DIR}/${BINARY_FILE}" ]; then
        if [ "$FORCE" = "false" ]; then
            warn "${BINARY_NAME} is already installed at ${INSTALL_DIR}/${BINARY_FILE}"
            warn "Use --force to reinstall"
            return 1
        fi
        info "Forcing reinstallation (--force specified)"
    fi
    return 0
}

# Main installation logic
install_binary() {
    download_url="$1"; os="$2"; tag="$3"

    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would download: $download_url"
        info "[DRY RUN] Would install to: ${INSTALL_DIR}/${BINARY_FILE}"
        return 0
    fi

    # Windows releases are zip archives, which tar cannot read
    if [ "$os" = "windows" ] && ! command_exists unzip; then
        error "unzip is required to extract the Windows release; install it, or download the .zip from the releases page"
        exit 1
    fi

    # Prepare temp workspace
    tmp_archive=$(mktemp)
    tmp_dir=$(mktemp -d)

    info "Downloading ${BINARY_NAME} ${tag}..."
    if ! download_file "$download_url" "$tmp_archive"; then
        rm -f "$tmp_archive"; rm -rf "$tmp_dir"
        error "Failed to download archive from: $download_url"
        exit 1
    fi

    if [ ! -s "$tmp_archive" ]; then
        rm -f "$tmp_archive"; rm -rf "$tmp_dir"
        error "Downloaded file is empty"
        exit 1
    fi

    # Extract archive
    info "Extracting archive..."
    if [ "$os" = "windows" ]; then
        unzip -q -o "$tmp_archive" -d "$tmp_dir"
    else
        tar -xzf "$tmp_archive" -C "$tmp_dir"
    fi

    # Locate binary inside the extracted directory
    src_binary="${tmp_dir}/${BINARY_FILE}"

    if [ ! -f "$src_binary" ]; then
        rm -f "$tmp_archive"; rm -rf "$tmp_dir"
        error "Binary '${BINARY_FILE}' not found inside the archive"
        exit 1
    fi

    # Install
    mkdir -p "$INSTALL_DIR"
    mv "$src_binary" "${INSTALL_DIR}/${BINARY_FILE}"
    chmod +x "${INSTALL_DIR}/${BINARY_FILE}"

    # Cleanup
    rm -f "$tmp_archive"
    rm -rf "$tmp_dir"

    success "${BINARY_NAME} has been installed to ${INSTALL_DIR}/${BINARY_FILE}"
}

# Post-install verification
verify_installation() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would verify installation at: ${INSTALL_DIR}/${BINARY_FILE}"
        return 0
    fi

    if [ -x "${INSTALL_DIR}/${BINARY_FILE}" ]; then
        success "Installation verified"
    else
        error "Installation verification failed"
        exit 1
    fi

    # Warn if the install directory is not in PATH
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            warn "${INSTALL_DIR} is not in your PATH"
            info "Add to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
            info "  export PATH=\"\$PATH:${INSTALL_DIR}\""
            if [ "$1" = "windows" ]; then
                info "To run it from PowerShell or cmd too, add that directory to your Windows user PATH."
            fi
            ;;
    esac
}

# Entry point
main() {
    info "${BINARY_NAME} Installation Script"
    info "========================="

    parse_args "$@"
    check_download_tool

    os=$(detect_os)
    arch=$(detect_arch)
    BINARY_FILE=$(binary_file "$os")
    info "Detected platform: ${os}/${arch}"

    # Resolve version
    tag="$VERSION"
    if [ "$tag" = "latest" ]; then
        info "Fetching latest release..."
        tag=$(get_latest_tag)
    else
        # Release tags carry no "v" prefix, so "v1.0.0" names the tag "1.0.0"
        tag="${tag#v}"
    fi
    info "Version: ${tag}"

    download_url=$(build_download_url "$tag" "$os" "$arch")

    check_existing_installation || exit 0

    install_binary "$download_url" "$os" "$tag"
    verify_installation "$os"

    info ""
    success "Installation complete! Run '${BINARY_NAME} --help' to get started."
}

main "$@"
