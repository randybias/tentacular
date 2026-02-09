#!/usr/bin/env bash
# install.sh — Install gVisor (runsc) on a k0s node (ARM64)
#
# This script is idempotent and safe to re-run.
# It downloads runsc + containerd-shim-runsc-v1, installs them to /usr/local/bin/,
# creates the containerd config for k0s, and restarts containerd.
#
# Usage: sudo bash deploy/gvisor/install.sh
set -euo pipefail

ARCH=$(uname -m)
INSTALL_DIR="/usr/local/bin"
K0S_CONTAINERD_DIR="/etc/k0s/containerd.d"

echo "=== gVisor Installer for k0s ==="

# Check root
if [[ $EUID -ne 0 ]]; then
  echo "ERROR: This script must be run as root (sudo)."
  exit 1
fi

# Validate architecture (gVisor supports x86_64 and aarch64)
if [[ "$ARCH" != "x86_64" && "$ARCH" != "aarch64" ]]; then
  echo "ERROR: Unsupported architecture: $ARCH"
  exit 1
fi

# Download runsc if not present or if updating
if command -v runsc &>/dev/null; then
  echo "runsc already installed: $(runsc --version 2>&1 | head -1)"
  echo "Re-downloading to ensure latest version..."
fi

echo "Downloading gVisor for ${ARCH}..."
GVISOR_URL="https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}"

wget -nv "${GVISOR_URL}/runsc" "${GVISOR_URL}/runsc.sha512" \
  "${GVISOR_URL}/containerd-shim-runsc-v1" "${GVISOR_URL}/containerd-shim-runsc-v1.sha512"
sha512sum -c runsc.sha512 -c containerd-shim-runsc-v1.sha512
rm -f *.sha512

chmod a+rx runsc containerd-shim-runsc-v1
mv runsc containerd-shim-runsc-v1 "${INSTALL_DIR}/"

echo "Installed: ${INSTALL_DIR}/runsc"
echo "Installed: ${INSTALL_DIR}/containerd-shim-runsc-v1"

# Verify binaries
"${INSTALL_DIR}/runsc" --version

# Create k0s containerd config
mkdir -p "${K0S_CONTAINERD_DIR}"

cat > "${K0S_CONTAINERD_DIR}/gvisor.toml" <<'TOML'
# gVisor runtime configuration for k0s containerd
# k0s automatically merges files in /etc/k0s/containerd.d/ into the containerd config
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
TOML

echo "Created ${K0S_CONTAINERD_DIR}/gvisor.toml"

# Restart k0s worker (containerd)
# k0s embeds containerd, so we restart the k0s service
if systemctl is-active --quiet k0sworker; then
  echo "Restarting k0sworker..."
  systemctl restart k0sworker
  echo "k0sworker restarted."
elif systemctl is-active --quiet k0scontroller; then
  echo "Restarting k0scontroller..."
  systemctl restart k0scontroller
  echo "k0scontroller restarted."
else
  echo "WARNING: Could not find k0s service to restart."
  echo "You may need to manually restart containerd or the k0s service."
fi

echo ""
echo "=== gVisor installation complete ==="
echo "Next steps:"
echo "  1. kubectl apply -f deploy/gvisor/runtimeclass.yaml"
echo "  2. kubectl apply -f deploy/gvisor/test-pod.yaml"
echo "  3. kubectl logs gvisor-test"
