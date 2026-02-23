#!/bin/sh
# Install tntc — the Tentacular CLI + Deno engine
# Usage: curl -fsSL https://raw.githubusercontent.com/randybias/tentacular/main/install.sh | sh
set -e

GITHUB_REPO="randybias/tentacular"
BINARY="tntc"
: ${TNTC_INSTALL_DIR:="$HOME/.local/bin"}
: ${TNTC_ENGINE_DIR:="$HOME/.tentacular/engine"}
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

_install_engine() {
    VERSION="$1"
    # Strip leading 'v' for the tarball directory name (v0.1.2 → 0.1.2)
    VERSION_CLEAN="${VERSION#v}"
    TARBALL_URL="https://github.com/${GITHUB_REPO}/archive/refs/tags/${VERSION}.tar.gz"

    echo "Installing engine ${VERSION}..."
    mkdir -p "${TNTC_ENGINE_DIR}"

    # Download source tarball and extract only the engine/ subdirectory.
    # --strip-components=2 removes 'tentacular-<ver>/engine/' leaving bare filenames,
    # which are then placed directly into TNTC_ENGINE_DIR.
    if command -v curl >/dev/null 2>&1; then
        curl -sSLf "${TARBALL_URL}" | tar -xz \
            --strip-components=2 \
            -C "${TNTC_ENGINE_DIR}" \
            "tentacular-${VERSION_CLEAN}/engine"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O - "${TARBALL_URL}" | tar -xz \
            --strip-components=2 \
            -C "${TNTC_ENGINE_DIR}" \
            "tentacular-${VERSION_CLEAN}/engine"
    else
        echo "Error: curl or wget is required" >&2; exit 1
    fi

    echo "Installed: ${TNTC_ENGINE_DIR}"
}

main() {
    OS=$(_detect_os)
    ARCH=$(_detect_arch)
    VERSION=${TNTC_VERSION:-$(_stable_version)}

    # Install tntc binary
    URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY}_${OS}_${ARCH}"

    echo "Downloading tntc ${VERSION} (${OS}/${ARCH})..."
    mkdir -p "${TNTC_INSTALL_DIR}"
    _fetch "${URL}" > "${TNTC_INSTALL_DIR}/${BINARY}"
    chmod 755 "${TNTC_INSTALL_DIR}/${BINARY}"
    echo "Installed: ${TNTC_INSTALL_DIR}/${BINARY}"

    # Install Deno engine (required for tntc test and tntc dev)
    _install_engine "${VERSION}"

    case ":${PATH}:" in
        *":${TNTC_INSTALL_DIR}:"*) ;;
        *) echo "Note: add ${TNTC_INSTALL_DIR} to your PATH" ;;
    esac

    echo ""
    echo "Installation complete. Run: tntc version"
}

main
