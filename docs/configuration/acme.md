# ACME / Let's Encrypt

Automatic HTTPS certificate management.

## Setup

### 1. Generate Configuration

```bash
traefikctl config --generate --acme --acme-email your@email.com
```

This creates:

```yaml
# Added to /etc/traefik/traefik.yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: your@email.com
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
```

### 2. Create ACME Storage

```bash
sudo touch /etc/traefik/acme.json
sudo chmod 600 /etc/traefik/acme.json
```

### 3. Restart Traefik

```bash
traefikctl service restart
```

### 4. Add HTTPS Routes

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

## Challenge Types

### HTTP Challenge (Default)

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      httpChallenge:
        entryPoint: web
```

Requires port 80 to be accessible.

### TLS-ALPN Challenge

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      tlsChallenge: true
```

Requires port 443 to be accessible.

### DNS Challenge

For wildcard certificates:

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: your@email.com
      storage: /etc/traefik/acme.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1"
          - "8.8.8.8"
```

Supported providers:

| Provider | Flag Value |
|----------|------------|
| CloudFlare | `cloudflare` |
| Route53 (AWS) | `route53` |
| DigitalOcean | `digitalocean` |
| GoDaddy | `godaddy` |
| OVH | `ovh` |

Environment variables required:

```bash
# CloudFlare
export CLOUDFLARE_EMAIL=your@email.com
export CLOUDFLARE_API_KEY=your_api_key

# AWS Route53
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret
```

## Wildcard Certificates

```bash
traefikctl resource add \
  --name wildcard \
  --domain "*.example.com" \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

Requires DNS challenge.

## Viewing Certificates

```bash
# View certificate info
sudo cat /etc/traefik/acme.json | jq .

# Or decode
sudo openssl x509 -in /etc/traefik/acme.json -text -noout
```

## Certificate Renewal

Let's Encrypt certificates auto-renew:

- **Validity**: 90 days
- **Renewal**: Starts at 60 days (30 days before expiry)
- **Storage**: `/etc/traefik/acme.json`

Monitor renewal:

```bash
traefikctl logs | grep -i acme
```

## Troubleshooting

### Challenge Fails

Check logs:

```bash
traefikctl logs | grep -i "acme\|challenge"
```

Common issues:

| Error | Solution |
|-------|----------|
| Connection refused | Open port 80/443 |
| DNS not propagated | Wait for DNS propagation |
| Rate limited | Wait and retry |

### Certificate Not Issued

```bash
# Check challenge
curl -I http://app.example.com/.well-known/acme-challenge/test
```

### Debug Mode

Enable debug logging:

```yaml
log:
  level: DEBUG
```

## Multiple Domains

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`) || Host(`www.example.com`)"
      tls:
        certResolver: letsencrypt
```
