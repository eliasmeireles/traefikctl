# HTTPS Setup

Complete guide to enabling HTTPS with Let's Encrypt.

## Prerequisites

- Domain name pointing to your server
- Port 80 and 443 open
- traefikctl installed

## Quick Setup

### 1. Install Traefik

```bash
traefikctl install
```

### 2. Generate Config with ACME

```bash
traefikctl config --generate --acme --acme-email admin@example.com
```

### 3. Create ACME Storage

```bash
sudo touch /etc/traefik/acme.json
sudo chmod 600 /etc/traefik/acme.json
```

### 4. Install Service

```bash
traefikctl service install
traefikctl service start
```

### 5. Add HTTPS Route

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

## Understanding the Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     Let's Encrypt Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Client                    Traefik                Let's      │
│  Request                  Server               Encrypt      │
│     │                       │                    │          │
│     │  1. HTTPS Request     │                    │          │
│     │──────────────────────>│                    │          │
│     │                       │                    │          │
│     │  2. ACME Challenge    │                    │          │
│     │<──────────────────────│  3. GET /.well-   │          │
│     │                       │──────> known/acme- │          │
│     │                       │     challenge/xxx  │          │
│     │                       │                    │          │
│     │  4. Validates         │                    │─────────>│
│     │  challenge           │                    │          │
│     │                       │  5. Issues cert   │<──────────│
│     │                       │<──────────────────│          │
│     │  6. HTTPS Response   │                    │          │
│     │<──────────────────────│                    │          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## HTTP to HTTPS Redirect

### Option 1: Using Middleware

```bash
# Create redirect middleware
traefikctl middleware add --name redirect-https --type redirect-https

# Apply to HTTP route
traefikctl resource update --name myapp --middleware redirect-https
```

### Option 2: Separate Redirect Route

```bash
# Add redirect-only route on port 80
traefikctl resource add \
  --name myapp-redirect \
  --domain app.example.com \
  --address 127.0.0.1:80 \
  --middleware redirect-https

# Note: This route should NOT have a real backend
# Edit /etc/traefik/dynamic/routes.yaml:
# Set address to a dummy service that returns nothing
```

## Wildcard Certificates

### Requirements

- DNS provider supported by Traefik
- API key/token for DNS provider
- Traefik v2.9+ for wildcard support

### CloudFlare Example

```bash
# Set environment variables
export CLOUDFLARE_EMAIL="your@email.com"
export CLOUDFLARE_API_KEY="your_api_key"

# Configure traefik.yaml
```

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@example.com
      storage: /etc/traefik/acme.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1"
          - "8.8.8.8"
```

### Add Wildcard Route

```bash
traefikctl resource add \
  --name wildcard \
  --domain "*.example.com" \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

## Multiple Domains

### Single Certificate

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt

traefikctl resource add \
  --name myapp-www \
  --domain www.example.com \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

### Separate Certificates

```yaml
http:
  routers:
    app:
      rule: "Host(`app.example.com`)"
      tls:
        certResolver: letsencrypt
        domains:
          - main: app.example.com
            sans:
              - "www.app.example.com"
```

## Custom TLS Settings

### Enhanced TLS Configuration

Edit `/etc/traefik/traefik.yaml`:

```yaml
tls:
  options:
    default:
      minVersion: VersionTLS12
      maxVersion: VersionTLS13
      cipherSuites:
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
        - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
```

### Apply TLS to Router

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      tls:
        options: default
        certResolver: letsencrypt
```

## Certificate Management

### View Certificates

```bash
# List from acme.json
sudo cat /etc/traefik/acme.json | jq '.[].certificate' | head -20
```

### Force Renewal

```bash
# Restart to trigger renewal check
traefikctl service restart

# Monitor logs
traefikctl logs | grep -i acme
```

### Backup Certificates

```bash
sudo cp /etc/traefik/acme.json /backup/acme.json.$(date +%Y%m%d)
```

## Troubleshooting

### Certificate Not Issued

```bash
# Check logs
traefikctl logs | grep -E "acme|challenge"

# Verify DNS
dig app.example.com

# Check ports
curl -I http://app.example.com/.well-known/acme-challenge/test
```

### Let's Encrypt Rate Limits

| Limit | Limit |
|-------|-------|
| 50 certificates per registered domain per week | 5 duplicate certificate limit per week |

### Common Issues

| Issue | Solution |
|-------|----------|
| Port 80 blocked | Open firewall: `sudo ufw allow 80` |
| DNS not propagated | Wait or use DNS challenge |
| Wrong email | Update in traefik.yaml |
| Rate limited | Wait 1 week or use staging |
