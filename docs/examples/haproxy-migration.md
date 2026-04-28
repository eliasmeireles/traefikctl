# HAProxy Migration

Convert existing HAProxy configurations to Traefik.

## Prerequisites

```bash
# Install traefikctl
curl -fsSL https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash

# Install Traefik
traefikctl install

# Generate initial config
traefikctl config --generate
```

## Basic Conversion

```bash
# Convert HAProxy config
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg
```

This creates Traefik config files in `/etc/traefik/dynamic/`.

## HAProxy to Traefik Mapping

### Frontends

```haproxy
# HAProxy
frontend http_front
    bind :80
    default_backend web_servers
    acl is_api path_beg /api
    use_backend api_servers if is_api
```

```yaml
# Traefik
http:
  routers:
    http_front:
      rule: "Host(`*`)"
      service: web_servers
      
    api_front:
      rule: "PathPrefix(`/api`)"
      service: api_servers
```

### Backends

```haproxy
# HAProxy
backend web_servers
    balance roundrobin
    server web1 10.0.0.2:8080 check inter 2000 rise 2 fall 3
    server web2 10.0.0.3:8080 check inter 2000 rise 2 fall 3
    option httpchk GET /health
```

```yaml
# Traefik
http:
  services:
    web_servers:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
          - url: "http://10.0.0.3:8080"
```

### TCP (Databases)

```haproxy
# HAProxy
listen postgres
    bind :5432
    mode tcp
    server db1 10.0.0.10:5432 check
    server db2 10.0.0.11:5432 check backup
```

```yaml
# Traefik
# Add to traefik.yaml entryPoints:
#   postgres:
#     address: ":5432"

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

### SSL Termination

```haproxy
# HAProxy
frontend https_front
    bind :443 ssl crt /etc/ssl/certs/server.pem
    default_backend web_servers
```

```yaml
# Traefik
http:
  routers:
    https_front:
      rule: "Host(`*`)"
      entryPoints:
        - websecure
      tls: {}
      service: web_servers
```

### ACLs Reference

| HAProxy | Traefik |
|---------|---------|
| `hdr(host) -i example.com` | `Host(\`example.com\`)` |
| `path_beg /api` | `PathPrefix(\`/api\`)` |
| `path_end .jpg` | `Path(\`.*\.jpg\`)` |
| `req.hdr(X-Custom) -m str value` | `Header(\`X-Custom\`, \`value\`)` |
| `method POST` | `Method(\`POST\`)` |
| `ssl_fc` | `ClientIP(\`...\`)` |

## Advanced: Custom Output

### Split by Frontend

```bash
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --split
```

Creates:
```
/etc/traefik/dynamic/
├── frontend-http.yaml
├── frontend-https.yaml
├── backend-web.yaml
└── backend-api.yaml
```

### Preview Without Writing

```bash
# Output to stdout
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --dry-run

# Output to specific directory
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --output-dir /tmp/preview
```

## Manual Edits

After conversion, you may need to manually add:

### TLS Certificates

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      tls:
        certResolver: letsencrypt
```

### Middlewares

```yaml
http:
  routers:
    api:
      rule: "PathPrefix(`/api`)"
      middlewares:
        - rate-limit
        - auth
```

### Custom Services

```yaml
http:
  services:
    api-weighted:
      weighted:
        services:
          - name: api-v1
            weight: 90
          - name: api-v2
            weight: 10
```

## Post-Migration Checklist

- [ ] Review generated configs
- [ ] Add TLS/ACME configuration
- [ ] Configure entrypoints
- [ ] Add missing middlewares
- [ ] Test all routes: `traefikctl resource check`
- [ ] Enable service: `traefikctl service restart`
- [ ] Monitor logs: `traefikctl logs`
- [ ] Verify HTTPS certificates

## Limitations

Some HAProxy features need manual migration:

- [ ] Complex ACL combinations
- [ ] HTTP/2 (requires Traefik v3+)
- [ ] Dynamic backend updates
- [ ] SRV record discovery
- [ ] Some stick tables
