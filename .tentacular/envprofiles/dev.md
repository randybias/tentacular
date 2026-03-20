# Cluster Environment Profile: dev
Generated: 2026-03-20T17:03:35Z | K8s: v1.34.2+k0s | Distribution: k0s

## Identity
- **Version:** v1.34.2+k0s
- **Distribution:** k0s
- **Nodes:** 1

## Runtime
- **gVisor:** available ✓
- **RuntimeClasses:** gvisor (runsc), wasmtime-spin-v2 (spin)

## Networking
- **CNI:** kube-router
- **NetworkPolicy:** supported, in use
- **Egress control:** supported
- **Ingress:** traefik

## Storage
| Name | Provisioner | Default | Reclaim | RWX (inferred) |
|------|-------------|---------|---------|----------------|
| local-path | rancher.io/local-path | ✓ | Delete | ✗ |

## Extensions
- ✗ Istio
- ✓ cert-manager
- ✗ Prometheus Operator
- ✗ External Secrets
- ✗ ArgoCD
- ✓ Gateway API
- ✓ Metrics Server
- Other CRD groups: autopilot.k0sproject.io, core.spinkube.dev, helm.k0sproject.io, hub.traefik.io, kro.run, spire.spiffe.io, traefik.io

## Namespace: tentacular-dev
- **Pod Security:** restricted
- **CPU quota:** request  / limit 4
- **Memory quota:** request  / limit 8Gi
- **Max pods:** 20
- **Default CPU:** request 100m / limit 500m
- **Default Memory:** request 64Mi / limit 256Mi

## Agent Guidance
1. Use runtime_class: gvisor for untrusted workflow steps
2. No RWX-capable StorageClass inferred — avoid shared volume mounts across replicas (verify with cluster admin)
3. ResourceQuota active in namespace "tentacular-dev": CPU limit 4, memory limit 8Gi
4. Namespace enforces restricted PodSecurity — containers must run as non-root with no privilege escalation
5. cert-manager available — TLS certificates can be provisioned automatically
