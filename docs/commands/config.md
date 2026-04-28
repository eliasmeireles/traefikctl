# config

Generate and view Traefik configurations.

## Synopsis

```bash
traefikctl config [flags]
```

## Examples

```bash
# Generate all configurations
traefikctl config --generate

# Generate and overwrite existing
traefikctl config --generate --force

# View current configurations
traefikctl config --view

# View without comments
traefikctl config --view --clean

# Generate with Let's Encrypt
traefikctl config --generate --acme --acme-email admin@example.com
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--generate` | `false` | Generate static and dynamic configurations |
| `--view` | `false` | Show current configurations |
| `--clean` | `false` | Strip comments (use with `--view`) |
| `--force` | `false` | Overwrite existing files |
| `--acme` | `false` | Append Let's Encrypt ACME config |
| `--acme-email` | (required) | Email for Let's Encrypt registration |

## Generated Files

### Static Configuration

Location: `/etc/traefik/traefik.yaml`

Contains:
- Entry points (web:80, websecure:443)
- API and dashboard settings
- TLS configuration
- File provider with directory watching

### Dynamic Configuration

Location: `/etc/traefik/dynamic/`

Contains:
- HTTP routers and services
- TCP routers and services
- Middlewares

## ACME/Let's Encrypt

```bash
traefikctl config --generate --acme --acme-email your@email.com
```

This appends:

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: your@email.com
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
```

## Customization

After generation, you can manually edit:
- `/etc/traefik/traefik.yaml` - Static settings
- `/etc/traefik/dynamic/*.yaml` - Route configurations
