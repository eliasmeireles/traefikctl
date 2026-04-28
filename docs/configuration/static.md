# Static Configuration

Core Traefik configuration file.

## Location

`/etc/traefik/traefik.yaml`

## Generate with traefikctl

```bash
traefikctl config --generate
```

## Full Example

```yaml
log:
  level: INFO
  filePath: /var/log/traefik/traefik.log
  format: common

accessLog:
  filePath: /var/log/traefik/access.log
  format: common

api:
  dashboard: true
  insecure: false

entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
  postgres:
    address: ":5432"
  redis:
    address: ":6379"

providers:
  file:
    directory: /etc/traefik/dynamic/
    watch: true

tls:
  options:
    default:
      minVersion: VersionTLS12
      cipherSuites:
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
        - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
```

## Configuration Sections

### Entry Points

Define network ports Traefik listens on:

```yaml
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
```

### API Configuration

```yaml
api:
  dashboard: true      # Enable dashboard
  insecure: false     # Don't expose on port 8080
```

> For dashboard, also configure a router:
> ```bash
> traefikctl resource add --name dashboard --domain dashboard.example.com --address 127.0.0.1:8080
> ```

### Logging

```yaml
log:
  level: DEBUG        # DEBUG, INFO, WARN, ERROR
  filePath: /var/log/traefik/traefik.log
  format: json        # common or json
```

### Access Log

```yaml
accessLog:
  filePath: /var/log/traefik/access.log
  format: common
  bufferingSize: 100
```

### TLS Options

```yaml
tls:
  options:
    default:
      minVersion: VersionTLS12
      maxVersion: VersionTLS13
      cipherSuites:
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
```

## Entrypoint Types

### TCP Entrypoint

For non-HTTP protocols:

```yaml
entryPoints:
  postgres:
    address: ":5432"
    tcp:
      udsName: /var/run/postgresql.sock
```

## Hot Reload

The file provider watches for changes:

```yaml
providers:
  file:
    directory: /etc/traefik/dynamic/
    watch: true
```
