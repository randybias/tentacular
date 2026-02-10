# gVisor Setup

gVisor provides kernel-level syscall interception for defense-in-depth container sandboxing. It is recommended but optional.

## Installation

For clusters without gVisor:

```bash
# Install on k0s nodes
sudo bash deploy/gvisor/install.sh

# Apply the RuntimeClass
kubectl apply -f deploy/gvisor/runtimeclass.yaml

# Verify
kubectl apply -f deploy/gvisor/test-pod.yaml
kubectl logs gvisor-test
```

## Usage

gVisor is enabled by default during deployment. To deploy without gVisor:

```bash
pipedreamer deploy my-workflow --runtime-class ""
```

## Preflight Check

`pipedreamer cluster check` validates that the gVisor RuntimeClass exists. Missing gVisor is a warning, not a hard failure — workflows will still deploy but without kernel-level sandboxing.

See [architecture.md](architecture.md) for details on all five security layers.
