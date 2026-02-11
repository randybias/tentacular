# gVisor Setup for k0s

Install gVisor (runsc) on a k0s cluster for sandboxed container execution.

## Prerequisites

- k0s cluster running on ARM64 (also supports x86_64)
- Root access to the node
- `kubectl` configured to access the cluster

## Installation

### 1. Install runsc on the node

```bash
sudo bash deploy/gvisor/install.sh
```

This script:
- Downloads `runsc` and `containerd-shim-runsc-v1` for your architecture
- Installs binaries to `/usr/local/bin/`
- Creates `/etc/k0s/containerd.d/gvisor.toml` (k0s auto-merges this config)
- Restarts the k0s service to pick up the new runtime

The script is idempotent — safe to re-run.

### 2. Create the RuntimeClass

```bash
kubectl apply -f deploy/gvisor/runtimeclass.yaml
```

### 3. Verify with a test pod

```bash
kubectl apply -f deploy/gvisor/test-pod.yaml
kubectl wait --for=condition=Ready pod/gvisor-test --timeout=30s || true
kubectl logs gvisor-test
```

The logs should show gVisor kernel messages (e.g., `Starting gVisor...`) rather than Linux kernel messages.

### 4. Cleanup

```bash
kubectl delete pod gvisor-test
```

## How Tentacular uses gVisor

When deploying workflows, Tentacular sets `runtimeClassName: gvisor` on the pod spec. This runs workflow containers inside gVisor's sandboxed kernel, providing an additional layer of isolation for untrusted code execution.

Use `--runtime-class=""` with `tntc deploy` to disable gVisor for a deployment.

## Troubleshooting

- **Pod stuck in ContainerCreating**: Check `kubectl describe pod gvisor-test` for events. The runsc binary may not be installed correctly.
- **"runtime not found"**: Verify `/etc/k0s/containerd.d/gvisor.toml` exists and the k0s service was restarted.
- **Permission denied**: The install script must run as root (`sudo`).
