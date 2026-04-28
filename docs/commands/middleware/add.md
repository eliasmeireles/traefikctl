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

### IP Whitelist

```bash
traefikctl middleware add --name internal-only --type ip-whitelist \
  --opt sourcerange=192.168.1.0/24,10.0.0.0/8
```

```yaml
http:
  middlewares:
    internal-only:
      ipWhiteList:
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

### ip-whitelist

| Option | Required | Description |
|--------|----------|-------------|
| `sourcerange` | Yes | Comma-separated CIDR ranges |

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
