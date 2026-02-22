
## Derived Artifacts

### Secrets

- `github.token` → service=github, key=token
- `openai.api_key` → service=openai, key=api_key

### Egress Rules (NetworkPolicy)

| Host | Port | Protocol |
|------|------|----------|
| kube-dns.kube-system.svc.cluster.local | 53 | UDP |
| kube-dns.kube-system.svc.cluster.local | 53 | TCP |
| api.github.com | 443 | TCP |
| api.openai.com | 443 | TCP |

### Ingress Rules (NetworkPolicy)

| Port | Protocol | Trigger |
|------|----------|---------|
| 8080 | TCP | webhook |

