# resource copy

Duplicate an existing route with different configuration.

## Synopsis

```bash
traefikctl resource copy [flags]
```

## Examples

```bash
# Copy to staging with new domain
traefikctl resource copy --from myapp --name myapp-staging --domain staging.example.com

# Copy with same domain (creates duplicate)
traefikctl resource copy --from myapp --name myapp-copy

# Copy to different file
traefikctl resource copy --from myapp --name myapp --dest /etc/traefik/dynamic/backup.yaml

# Copy and change backend
traefikctl resource copy --from myapp --name myapp-v2 --address 10.0.0.5:8080
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--from` | Yes | Source router name |
| `--name` | Yes | Destination router name |
| `--domain` | No | New domain for destination |
| `--file` | No | Source dynamic config file |
| `--dest` | No | Destination file |

## What Gets Copied

| Item | Copied | Modified |
|------|--------|----------|
| Router rule | ✓ | If `--domain` provided |
| Service | ✓ | Uses new name |
| Middlewares | ✓ | References remain |
| TLS settings | ✓ | - |

## Service Name Generation

Copied routes get new service names:

```
Original: myapp (service: myapp-svc)
Copy:     myapp-staging (service: myapp-staging-svc)
```

## Use Cases

### Environment Clones

```bash
# Production to staging
traefikctl resource copy --from production-api --name staging-api \
  --domain staging.example.com
```

### Version Deployments

```bash
# v1 to v2
traefikctl resource copy --from api-v1 --name api-v2 \
  --address 10.0.0.6:3000
```

### Regional Deployments

```bash
# US to EU
traefikctl resource copy --from app-us --name app-eu \
  --domain app.example.eu
```
