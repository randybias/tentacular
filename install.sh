#!/bin/sh
# Install tntc — the Tentacular CLI
# Usage: curl -fsSL https://raw.githubusercontent.com/randybias/tentacular/main/install.sh | sh
set -e

GITHUB_REPO="randybias/tentacular"
BINARY="tntc"
: ${TNTC_INSTALL_DIR:="$HOME/.local/bin"}
: ${TNTC_VERSION:=""}

_fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -sSLf "$@"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O - "$@"
    else
        echo "Error: curl or wget is required" >&2; exit 1
    fi
}

_detect_os() {
    os=$(uname | tr '[:upper:]' '[:lower:]')
    case "$os" in
        darwin|linux) echo "$os" ;;
        *) echo "Unsupported OS: $os" >&2; exit 1 ;;
    esac
}

_detect_arch() {
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
    esac
}

_stable_version() {
    _fetch "https://raw.githubusercontent.com/${GITHUB_REPO}/main/stable.txt"
}

main() {
    OS=$(_detect_os)
    ARCH=$(_detect_arch)
    VERSION=${TNTC_VERSION:-$(_stable_version)}

    URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY}_${OS}_${ARCH}"

    echo "Downloading tntc ${VERSION} (${OS}/${ARCH})..."
    mkdir -p "${TNTC_INSTALL_DIR}"
    _fetch "${URL}" > "${TNTC_INSTALL_DIR}/${BINARY}"
    chmod 755 "${TNTC_INSTALL_DIR}/${BINARY}"

    echo "Installed: ${TNTC_INSTALL_DIR}/${BINARY}"

    case ":${PATH}:" in
        *":${TNTC_INSTALL_DIR}:"*) ;;
        *) echo "Note: add ${TNTC_INSTALL_DIR} to your PATH" ;;
    esac

    echo "Run: tntc version"
}

main
