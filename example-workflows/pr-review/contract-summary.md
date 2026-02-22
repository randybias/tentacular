
## Derived Artifacts

### Secrets

- `github.token` → service=github, key=token
- `anthropic.api_key` → service=anthropic, key=api_key

### Egress Rules (NetworkPolicy)

| Host | Port | Protocol |
|------|------|----------|
| kube-dns.kube-system.svc.cluster.local | 53 | UDP |
| kube-dns.kube-system.svc.cluster.local | 53 | TCP |
| api.github.com | 443 | TCP |
| api.anthropic.com | 443 | TCP |

### Ingress Rules (NetworkPolicy)

| Port | Protocol | Trigger |
|------|----------|---------|
| 8080 | TCP | webhook |

