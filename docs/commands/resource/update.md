# resource update

Update an existing route's configuration.

## Synopsis

```bash
traefikctl resource update [flags]
```

## Examples

```bash
# Update backend address
traefikctl resource update --name myapp --address 10.0.0.5:8080

# Update domain
traefikctl resource update --name myapp --domain new.example.com

# Update both
traefikctl resource update --name myapp --address 10.0.0.5:8080 --domain new.example.com

# Add middleware
traefikctl resource update --name myapp --middleware new-middleware

# Update specific file
traefikctl resource update --name myapp --file /etc/traefik/dynamic/custom.yaml
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Router name to update |
| `--address` | No | New server address (`ip:port`) |
| `--domain` | No | New domain for router rule |
| `--middleware` | No | Middleware(s) to attach |
| `--file` | No | Dynamic config file |

## What Can Be Updated

| Field | Description |
|-------|-------------|
| `address` | Backend server address |
| `domain` | Host rule domain |
| `middlewares` | Attached middleware list |

## What Requires Recreation

These fields require removing and re-adding the route:
- Route type (HTTP to TCP)
- Entrypoint
- TLS settings

## Updating Middleware

```bash
# Replace all middlewares
traefikctl resource update --name myapp --middleware auth,rate-limit

# Add new middleware (append)
# Note: Manual edit required for append
```

## Notes

- Changes are applied immediately via Traefik hot reload
- Always validate changes with `traefikctl resource check`
