# haproxy export

Convert HAProxy configuration to Traefik format.

## Synopsis

```bash
traefikctl haproxy export [flags]
```

## Examples

```bash
# Basic export
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg

# Custom output directory
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --output-dir /tmp/traefik

# Split into multiple files
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --split

# From base64 (for pipelines)
cat /etc/haproxy/haproxy.cfg | base64 -w0 | xargs -I {} traefikctl haproxy export --base64 {}

# Skip entrypoint configuration
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg --no-apply-entrypoints
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | (required*) | Path to HAProxy config |
| `--base64` | - | Base64-encoded config |
| `--output-dir` | `/etc/traefik/dynamic` | Output directory |
| `--split` | `false` | Split by frontend/listen |
| `--no-apply-entrypoints` | `false` | Skip TCP entrypoints |

> \* Either `--file` or `--base64` is required.

## Conversion Rules

### Frontend/Listen (HTTP)

| HAProxy | Traefik |
|---------|---------|
| `frontend <name>` | `http.routers.<name>` |
| `bind :80` | Entrypoint `web` |
| `bind :443` | Entrypoint `websecure` |
| `use_backend <name>` | Service reference |
| `acl <name> <condition>` | `Rule` |

### Listen (TCP)

| HAProxy | Traefik |
|---------|---------|
| `listen <name>` | `tcp.routers.<name>` |
| `bind :5432` | Entrypoint `postgres` |
| `source 0.0.0.0:0 except <ip>` | `Rule` |

### Backends

| HAProxy | Traefik |
|---------|---------|
| `backend <name>` | `http.services.<name>` |
| `server <name> <ip:port>` | `loadBalancer.servers` |

## Example Conversion

### HAProxy

```haproxy
frontend http_front
    bind :80
    default_backend web_servers

backend web_servers
    server web1 10.0.0.2:8080 check
    server web2 10.0.0.3:8080 check
```

### Traefik

```yaml
http:
  routers:
    http_front:
      rule: "Host(`*`)"
      entryPoints:
        - web
      service: web_servers
  services:
    web_servers:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
          - url: "http://10.0.0.3:8080"
```

## ACL Conversion

| HAProxy ACL | Traefik Rule |
|-------------|--------------|
| `hdr(host) -i example.com` | `Host(\`example.com\`)` |
| `path_beg /api` | `PathPrefix(\`/api\`)` |
| `path_end .jpg` | `Path(\`.*\.jpg\`)` |
| `hdr_reg(User-Agent) -i curl` | `Header(\`User-Agent\`, \`curl\`)` |

## Post-Export Steps

1. Review generated configs
2. Add TLS settings manually
3. Configure entrypoints if needed
4. Test with `traefikctl resource check`
5. Enable routes

## Limitations

Not all HAProxy features are supported:

- [ ] Complex ACL combinations
- [ ] HTTP/2 mode (Traefik v3+ required)
- [ ] SRV records
- [ ] Dynamic backend updates
