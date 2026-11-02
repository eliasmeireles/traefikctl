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
| `--address` | One of address/service | - | Backend address (`ip:port`) |
| `--service` | One of address/service | - | Existing service in the matching protocol namespace; mutually exclusive with address |
| `--domain` | No | - | Hostnames for HTTP `Host` or TCP `HostSNI` (repeatable or comma-separated) |
| `--type` | No | `http` | Route type: `http` or `tcp` |
| `--entrypoint` | No | `web` | Traefik entrypoint |
| `--file` | No | - | Dynamic config file |
| `--middleware` | No | - | Middleware names (comma-separated) |
| `--redirect-https` | No | `false` | Add HTTP to HTTPS redirect |
| `--tls` | No | `false` | Enable TLS |
| `--passthrough` | No | `false` | TCP TLS passthrough; requires explicit domain or `*` and TLS entrypoint |
| `--priority` | No | `0` | HTTP/TCP router priority |
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

TCP without a domain keeps ``HostSNI(`*`)`` and no TLS by default. Supplying a
specific hostname enables TLS matching. Multiple domains build a disjunction,
for example ``HostSNI(`a.example.com`) || HostSNI(`b.example.com`)``.
Domains must be hostnames without a scheme, port or path. Only TCP accepts `*`.

Passthrough requires a TLS entrypoint: port 443 or an entrypoint with `http.tls`
in `/etc/traefik/traefik.yaml`. When that static file is absent, `websecure` is
accepted as the conventional TLS entrypoint. `--passthrough` itself enables TLS;
`--tls` is not also required. HTTP rejects passthrough. TCP rejects the HTTP-only
`--redirect-https` and `--cert-resolver` options.

`--middleware` attaches middleware names in the router's protocol namespace.
Names use letters, digits, dots, underscores and hyphens and start with a letter
or digit. Provider-qualified service names are not supported by `--service`.

## VPN-only TCP TLS passthrough

Assume a TCP service named `shared-https` already exists in another YAML file
in the same dynamic provider directory. Restrict the route to VPN clients:

```bash
sudo traefikctl middleware add --name vpn-only --type ip-allowlist --protocol tcp \
  --opt sourcerange=10.99.0.0/24
sudo traefikctl resource add --name admin-vpn --type tcp --domain admin.example.com \
  --entrypoint websecure --passthrough --priority 100 --middleware vpn-only --service shared-https
```

The router and middleware may live in separate files. Together they produce:

```yaml
tcp:
  routers:
    admin-vpn:
      rule: HostSNI(`admin.example.com`)
      priority: 100
      entryPoints: [websecure]
      middlewares: [vpn-only]
      service: shared-https
      tls:
        passthrough: true
  middlewares:
    vpn-only:
      ipAllowList:
        sourceRange: [10.99.0.0/24]
```

TLS remains encrypted until the backend, which supplies the certificate.
The allowlist checks the connection's source IP; it must be visible to Traefik.
`--service` validates the matching protocol's service in neighboring `.yaml` or
`.yml` files and creates no service. Use `--file` to select the destination.
