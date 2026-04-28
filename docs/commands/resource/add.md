# resource add

Add a new HTTP or TCP route.

## Synopsis

```bash
traefikctl resource add [flags]
```

## Examples

```bash
# Basic HTTP route
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080

# HTTPS with Let's Encrypt
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080 \
  --tls --cert-resolver letsencrypt

# HTTP to HTTPS redirect
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080 \
  --redirect-https

# TCP route
traefikctl resource add --name postgres --address 10.0.0.10:5432 --type tcp \
  --entrypoint postgres

# With middleware
traefikctl resource add --name api --domain api.example.com --address 10.0.0.3:3000 \
  --entrypoint websecure --middleware rate-limit,auth
```

## Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--name` | Yes | - | Router and service name |
| `--address` | Yes | - | Backend address (`ip:port`) |
| `--domain` | No | - | Domain for host rule (`Host(...)`) |
| `--type` | No | `http` | Route type: `http` or `tcp` |
| `--entrypoint` | No | `web` | Traefik entrypoint |
| `--file` | No | - | Dynamic config file |
| `--middleware` | No | - | Middleware names (comma-separated) |
| `--redirect-https` | No | `false` | Add HTTP to HTTPS redirect |
| `--tls` | No | `false` | Enable TLS |
| `--cert-resolver` | No | - | Let's Encrypt resolver name |

## Generated Configuration

For `--name myapp --domain app.example.com --address 10.0.0.2:8080`:

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      entryPoints:
        - web
      service: myapp-svc
  services:
    myapp-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
```

## TCP Route Example

For `--name postgres --address 10.0.0.10:5432 --type tcp --entrypoint postgres`:

```yaml
tcp:
  routers:
    postgres:
      rule: "HostSNI(`*`)"
      entryPoints:
        - postgres
      service: postgres-svc
  services:
    postgres-svc:
      loadBalancer:
        servers:
          - address: "10.0.0.10:5432"
```

## Multiple Entrypoints

```bash
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080 \
  --entrypoint web,websecure
```

## Custom Priority

Routes with higher priority are matched first:

```bash
# Higher priority for API
traefikctl resource add --name api --domain example.com --address 10.0.0.3:3000 \
  --entrypoint websecure --priority 100
```

> Note: Priority must be set via manual edit after creation.
