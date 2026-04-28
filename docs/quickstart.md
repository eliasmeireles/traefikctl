# Quick Start

Get up and running with traefikctl in 5 minutes.

## 1. Install traefikctl

```bash
curl -fsSL https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash
```

## 2. Install Traefik

```bash
traefikctl install
```

This downloads Traefik, creates the necessary user and directories, and sets up permissions.

## 3. Generate Configuration

```bash
traefikctl config --generate
```

This creates:
- `/etc/traefik/traefik.yaml` - Main configuration
- `/etc/traefik/dynamic/` - Directory for route configurations

## 4. Add Your First Route

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080
```

This creates a new HTTP route that forwards requests to your backend.

## 5. Enable HTTPS

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --tls \
  --cert-resolver letsencrypt \
  --acme-email your@email.com
```

> **Note**: Make sure to append ACME configuration first:
> ```bash
> traefikctl config --generate --acme --acme-email your@email.com
> ```

## 6. Install as System Service

```bash
traefikctl service install
traefikctl service start
```

## 7. Monitor Status

```bash
# Check service status
traefikctl status

# View logs
traefikctl logs

# Check all routes
traefikctl resource list
```

## Common Workflows

### Adding Rate Limiting

```bash
# Create middleware
traefikctl middleware add --name myapp-limit --type rate-limit \
  --opt average=100 --opt burst=50

# Apply to route
traefikctl resource update --name myapp --middleware myapp-limit
```

### Adding Basic Authentication

```bash
# Generate password hash
htpasswd -nb admin mypassword

# Create middleware
traefikctl middleware add --name myapp-auth --type basic-auth \
  --opt users="admin:$$apr1$$H6uskkkW$$IgXLP6ewTrSuBkTrqE8wj/"

# Apply to route
traefikctl resource update --name myapp --middleware myapp-auth
```

### Migrating from HAProxy

```bash
traefikctl haproxy export --file /etc/haproxy/haproxy.cfg
```

## Next Steps

- **[Commands Reference](commands/index.md)** - Explore all available commands
- **[Configuration Guide](configuration/index.md)** - Customize your setup
- **[Examples](examples/index.md)** - Real-world use cases

## Environment

After setup, your environment looks like this:

```
┌──────────────┐
│   Internet   │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│    Traefik   │ :80, :443
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Your App    │ 10.0.0.2:8080
└──────────────┘
```

## Help

If you need help:

```bash
traefikctl --help
traefikctl <command> --help
```
