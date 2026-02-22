#!/usr/bin/env bash
set -e

GITHUB_REPO="randybias/tentacular"
BINARY_NAME="tntc"
: ${TNTC_INSTALL_DIR:="$HOME/.local/bin"}
: ${TNTC_VERSION:="latest"}
: ${TNTC_BUILD_FROM_SOURCE:="false"}

# ── Logging ──────────────────────────────────────────────────────────────────

info()  { echo '[INFO] ' "$@"; }
warn()  { echo '[WARN] ' "$@" >&2; }
fatal() { echo '[ERROR] ' "$@" >&2; exit 1; }

# ── OS / Arch detection ───────────────────────────────────────────────────────

setup_verify_os() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    case "${OS}" in
        darwin|linux) ;;
        *) fatal "Unsupported OS: ${OS}" ;;
    esac
}

setup_verify_arch() {
    ARCH=$(uname -m)
    case "${ARCH}" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *) fatal "Unsupported architecture: ${ARCH}" ;;
    esac
}

# ── Downloader ────────────────────────────────────────────────────────────────

verify_downloader() {
    command -v "$1" &>/dev/null || return 1
    DOWNLOADER="$1"
}

download() {
    local url="$1"
    local dest="$2"
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -fsSL -o "${dest}" "${url}"
    else
        wget -qO "${dest}" "${url}"
    fi
}

# ── Temp dir cleanup ──────────────────────────────────────────────────────────

setup_tmp() {
    TMP_DIR=$(mktemp -d -t tntc-install.XXXXXXXXXX)
    cleanup() {
        local code=$?
        set +e
        trap - EXIT
        rm -rf "${TMP_DIR}"
        exit ${code}
    }
    trap cleanup INT EXIT
}

# ── PATH check ────────────────────────────────────────────────────────────────

check_path() {
    if [[ ":${PATH}:" != *":${TNTC_INSTALL_DIR}:"* ]]; then
        warn "${TNTC_INSTALL_DIR} is not in your PATH."
        warn "Add the following to your shell profile:"
        warn "  export PATH=\"\${PATH}:${TNTC_INSTALL_DIR}\""
    fi
}

# ── Phase 1: Binary download from GitHub Releases ────────────────────────────

install_from_release() {
    info "Checking for GitHub Release..."

    local release_url
    if [ "${TNTC_VERSION}" = "latest" ]; then
        release_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    else
        release_url="https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${TNTC_VERSION}"
    fi

    local metadata_file="${TMP_DIR}/release.json"
    download "${release_url}" "${metadata_file}" 2>/dev/null || return 1

    # Check for a real release (not a 404 or empty)
    if ! grep -q '"tag_name"' "${metadata_file}" 2>/dev/null; then
        info "No published release found — falling back to source build."
        return 1
    fi

    VERSION=$(grep '"tag_name"' "${metadata_file}" | sed -E 's/.*"([^"]+)".*/\1/')
    info "Found release ${VERSION}"

    local archive="tntc_${OS}_${ARCH}.tar.gz"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${archive}"
    local checksum_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

    info "Downloading ${archive}..."
    download "${download_url}" "${TMP_DIR}/${archive}" || return 1

    info "Verifying checksum..."
    download "${checksum_url}" "${TMP_DIR}/checksums.txt" || return 1
    (cd "${TMP_DIR}" && grep "${archive}" checksums.txt | sha256sum -c --status 2>/dev/null) \
        || (cd "${TMP_DIR}" && grep "${archive}" checksums.txt | shasum -a 256 -c --status 2>/dev/null) \
        || { warn "Checksum verification failed"; return 1; }

    tar -xzf "${TMP_DIR}/${archive}" -C "${TMP_DIR}"
    install_binary "${TMP_DIR}/${BINARY_NAME}"
}

# ── Phase 2: Build from source ────────────────────────────────────────────────

install_from_source() {
    info "Building from source..."

    command -v go &>/dev/null  || fatal "Go is required to build from source. Install from https://go.dev/dl/"
    command -v git &>/dev/null || fatal "git is required to build from source."

    local go_version
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    info "Using Go ${go_version}"

    local clone_dir="${TMP_DIR}/tentacular"
    local clone_ref

    if [ "${TNTC_VERSION}" = "latest" ]; then
        clone_ref="main"
    else
        clone_ref="${TNTC_VERSION}"
    fi

    info "Cloning ${GITHUB_REPO}@${clone_ref}..."
    git clone --depth 1 --branch "${clone_ref}" \
        "https://github.com/${GITHUB_REPO}.git" "${clone_dir}" 2>/dev/null \
        || git clone --depth 1 "https://github.com/${GITHUB_REPO}.git" "${clone_dir}"

    info "Building ${BINARY_NAME}..."
    (
        cd "${clone_dir}"
        GOBIN="${TMP_DIR}" go install ./cmd/tntc
    )

    install_binary "${TMP_DIR}/${BINARY_NAME}"
}

# ── Install binary ────────────────────────────────────────────────────────────

install_binary() {
    local src="$1"
    mkdir -p "${TNTC_INSTALL_DIR}"
    cp "${src}" "${TNTC_INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${TNTC_INSTALL_DIR}/${BINARY_NAME}"
    info "Installed ${BINARY_NAME} → ${TNTC_INSTALL_DIR}/${BINARY_NAME}"
}

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
    setup_verify_os
    setup_verify_arch

    verify_downloader curl || verify_downloader wget \
        || fatal "curl or wget is required"

    setup_tmp

    if [ "${TNTC_BUILD_FROM_SOURCE}" = "true" ]; then
        install_from_source
    else
        install_from_release || install_from_source
    fi

    check_path

    info "Done. Run: tntc version"
}

main "$@"
