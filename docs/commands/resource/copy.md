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

## TCP routes and shared services

TCP routes support repeated or comma-separated `--domain` values, producing
`HostSNI` rules. Omitting the flag preserves the existing rule. TLS, middleware
references and priority are preserved when copying.

Additional options: `--type http|tcp` checks the existing router type;
`--entrypoint`, `--middleware` (repeatable or comma-separated), `--priority` and
`--passthrough` replace those settings. Explicit `--passthrough=false` disables
passthrough; `--priority=0` resets priority; `--middleware=` clears references.
Passthrough is TCP-only and requires a TLS entrypoint.

`--service` selects a verified existing service instead of `--address`. The two
flags are mutually exclusive. With neither flag, the existing backend is kept.
Copying a local service creates a service with the new route name; copying a
reference to a service in another file preserves the reference and verifies it
in the destination directory. Updating an external service's address requires
editing its owning file; the command reports an error instead of silently doing
nothing.

```bash
traefikctl resource copy --from admin-vpn --name admin-staging --type tcp \
  --domain admin.example.com --domain staging.example.com --passthrough --priority 100
```
