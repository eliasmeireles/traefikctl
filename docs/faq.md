# FAQ

Frequently Asked Questions about traefikctl.

## General

### What is traefikctl?

traefikctl is a CLI tool that simplifies managing Traefik reverse proxy configurations. It provides commands for installing Traefik, managing routes, configuring HTTPS, and more.

### What is Traefik?

Traefik is a modern reverse proxy and load balancer that supports multiple protocols (HTTP, TCP, UDP) and can automatically discover and configure routes to services.

### Do I need Traefik installed separately?

No, traefikctl can install Traefik for you with `traefikctl install`.

### What Linux distributions are supported?

All major Linux distributions: Ubuntu, Debian, CentOS, Fedora, RHEL, Arch Linux, etc.

### Can I use traefikctl with Docker?

traefikctl is designed for bare-metal/VM installations. For Docker, use Traefik's native labels and docker provider.

## Installation

### Installation fails with "permission denied"

Run with sudo:
```bash
sudo traefikctl install
```

### curl command fails

Try alternative installation:
```bash
wget -O- https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash
```

### How to update traefikctl?

```bash
traefikctl update
```

## Configuration

### Where is the config file?

Default: `/etc/traefikctl/config.yaml`

Use `--config` flag to specify a custom location.

### How to enable the dashboard?

```bash
traefikctl config --generate --acme --acme-email your@email.com
traefikctl service restart
```

Then add a route:
```bash
traefikctl resource add --name dashboard --domain traefik.example.com --address 127.0.0.1:8080
```

### How to change log level?

Edit `/etc/traefik/traefik.yaml`:
```yaml
log:
  level: DEBUG
```

### How to configure multiple domains?

```bash
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080
traefikctl resource add --name myapp-www --domain www.example.com --address 10.0.0.2:8080
```

## HTTPS / TLS

### HTTPS is not working

1. Check Let's Encrypt setup:
```bash
traefikctl logs | grep -i acme
```

2. Verify port 80 is accessible:
```bash
curl -I http://your-domain.com/.well-known/acme-challenge/test
```

3. Check certificate:
```bash
sudo cat /etc/traefik/acme.json
```

### How to use my own certificate?

Place certs in `/etc/traefik/` and reference in config:
```yaml
tls:
  certificates:
    - certFile: /etc/traefik/certs/example.com.crt
      keyFile: /etc/traefik/certs/example.com.key
```

### Certificate expired / renewal failed

```bash
traefikctl service restart
traefikctl logs | grep -i acme
```

## Routes

### How to add a route?

```bash
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080
```

### How to change the backend address?

```bash
traefikctl resource update --name myapp --address 10.0.0.5:8080
```

### How to remove a route?

```bash
traefikctl resource remove --name myapp
```

### Can I use paths instead of domains?

Edit the dynamic config file directly:

```yaml
http:
  routers:
    api:
      rule: "PathPrefix(`/api`)"
      service: api-svc
```

### How to enable load balancing?

Add multiple backends:
```bash
traefikctl resource backend add --name myapp --address 10.0.0.3:8080
traefikctl resource backend add --name myapp --address 10.0.0.4:8080
```

## Middleware

### How to add authentication?

```bash
# Generate password
htpasswd -nb admin password123

# Create middleware
traefikctl middleware add --name auth --type basic-auth \
  --opt users="admin:$$apr1$$H6uskkkW$$IgXLP6ewTrSuBkTrqE8wj/"

# Apply to route
traefikctl resource update --name myapp --middleware auth
```

### How to rate limit?

```bash
traefikctl middleware add --name limit --type rate-limit \
  --opt average=100 --opt burst=50

traefikctl resource update --name myapp --middleware limit
```

## Service Management

### How to restart Traefik?

```bash
traefikctl service restart
```

### How to reload without restart?

```bash
traefikctl service reload
```

### How to view logs?

```bash
traefikctl logs
traefikctl logs --access  # access log
traefikctl logs --service # from journal
```

### How to auto-start on boot?

```bash
sudo systemctl enable traefikctl
```

## Troubleshooting

### Traefik won't start

1. Check logs:
```bash
traefikctl logs
traefikctl service logs
```

2. Validate config:
```bash
/usr/local/bin/traefik validate --configfile /etc/traefik/traefik.yaml
```

3. Check permissions:
```bash
ls -la /etc/traefik/
```

### Route not working

1. Check route exists:
```bash
traefikctl resource list
```

2. Check backend connectivity:
```bash
traefikctl resource check --name myapp
```

3. Verify DNS:
```bash
dig app.example.com
```

### 502 Bad Gateway

1. Backend unreachable:
```bash
traefikctl resource check --name myapp
```

2. Wrong address:
```bash
traefikctl resource list
```

### 503 Service Unavailable

All backends down or health check failing.

### Permission denied errors

```bash
sudo chown -R traefik:traefik /etc/traefik
sudo chown -R traefik:traefik /var/log/traefik
```

## Getting Help

- GitHub Issues: https://github.com/eliasmeireles/traefikctl/issues
- Documentation: This documentation site
- Commands help: `traefikctl <command> --help`
