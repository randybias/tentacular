
## Derived Artifacts

### Secrets

- `openai.api_key` → service=openai, key=api_key
- `slack.webhook_url` → service=slack, key=webhook_url

### Egress Rules (NetworkPolicy)

| Host | Port | Protocol |
|------|------|----------|
| kube-dns.kube-system.svc.cluster.local | 53 | UDP |
| kube-dns.kube-system.svc.cluster.local | 53 | TCP |
| 0.0.0.0/0 | 443 | TCP |
| api.openai.com | 443 | TCP |
| hooks.slack.com | 443 | TCP |

### Ingress Rules (NetworkPolicy)

| Port | Protocol | Trigger |
|------|----------|---------|
| 8080 | TCP | webhook |

