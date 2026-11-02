# middleware add

Add a new middleware.

## Synopsis

```bash
traefikctl middleware add [flags]
```

## Examples

### HTTP to HTTPS Redirect

```bash
traefikctl middleware add --name redirect-to-https --type redirect-https
```

```yaml
http:
  middlewares:
    redirect-to-https:
      redirectScheme:
        scheme: https
        permanent: true
```

### Rate Limiting

```bash
traefikctl middleware add --name api-limit --type rate-limit \
  --opt average=100 --opt burst=50
```

```yaml
http:
  middlewares:
    api-limit:
      rateLimit:
        average: 100
        burst: 50
```

### Basic Authentication

```bash
# Generate password
htpasswd -nb admin password123

# Create middleware
traefikctl middleware add --name admin-auth --type basic-auth \
  --opt users="admin:$$apr1$$H6uskkkW$$IgXLP6ewTrSuBkTrqE8wj/"
```

```yaml
http:
  middlewares:
    admin-auth:
      basicAuth:
        users:
          - "admin:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/"
```

### Strip Prefix

```bash
traefikctl middleware add --name strip-api --type strip-prefix \
  --opt prefixes=/api,/v1
```

```yaml
http:
  middlewares:
    strip-api:
      stripPrefix:
        prefixes:
          - /api
          - /v1
```

### Add Prefix

```bash
traefikctl middleware add --name add-base --type add-prefix \
  --opt prefix=/api/v2
```

```yaml
http:
  middlewares:
    add-base:
      addPrefix:
        prefix: /api/v2
```

### IP Allowlist (HTTP and TCP)

```bash
traefikctl middleware add --name internal-only --type ip-allowlist \
  --opt sourcerange=192.168.1.0/24,10.0.0.0/8
```

```yaml
http:
  middlewares:
    internal-only:
      ipAllowList:
        sourceRange:
          - 192.168.1.0/24
          - 10.0.0.0/8
```

### Custom Headers

```bash
traefikctl middleware add --name security-headers --type headers \
  --opt sslredirect=true \
  --opt stsseconds=31536000 \
  --opt contenttypenosniff=true
```

```yaml
http:
  middlewares:
    security-headers:
      headers:
        sslRedirect: true
        stsSeconds: 31536000
        contentTypeNosniff: true
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Middleware name |
| `--type` | Yes | Middleware type |
| `--protocol` | No | `http` (default) or `tcp` |
| `--file` | No | Dynamic config file |
| `--opt` | Varies | Options as `key=value` |

## Middleware Types Reference

### redirect-https

| Option | Default | Description |
|--------|---------|-------------|
| `scheme` | `https` | Redirect scheme |
| `permanent` | `true` | 301 redirect |

### rate-limit

| Option | Default | Description |
|--------|---------|-------------|
| `average` | `100` | Average requests per second |
| `burst` | `50` | Burst allowance |

### basic-auth

| Option | Required | Description |
|--------|----------|-------------|
| `users` | Yes | Comma-separated htpasswd entries |

### strip-prefix

| Option | Required | Description |
|--------|----------|-------------|
| `prefixes` | Yes | Comma-separated path prefixes |

### add-prefix

| Option | Default | Description |
|--------|---------|-------------|
| `prefix` | - | Path prefix to add |

### ip-allowlist / ip-whitelist

| Option | Required | Description |
|--------|----------|-------------|
| `sourcerange` | Yes | Comma-separated CIDR ranges or individual IPv4/IPv6 addresses |

### headers

| Option | Default | Description |
|--------|---------|-------------|
| `sslredirect` | `false` | Redirect HTTP to HTTPS |
| `stsseconds` | `0` | HSTS max-age |
| `contenttypenosniff` | `false` | X-Content-Type-Options: nosniff |
| `browserxssfilter` | `false` | X-XSS-Protection |
| `forceSTSHeader` | `false` | Force HSTS header |
| `stsincludeSubdomains` | `false` | Include subdomains |
| `stspreload` | `false` | HSTS preload |

`ip-allowlist` writes Traefik v3 `ipAllowList` for HTTP and TCP. Invalid IPs and
CIDRs are rejected. TCP supports only `ip-allowlist`; HTTP-only middleware types
are rejected when `--protocol tcp` is specified.

`ip-whitelist` is a deprecated HTTP compatibility alias. This command does not
detect the running Traefik version, so it retains the legacy `ipWhiteList` YAML
key. Use `ip-allowlist` with Traefik v3.

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
