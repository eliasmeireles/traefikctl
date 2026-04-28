# Configuration Guide

Complete guide to configuring Traefik with traefikctl.

## Configuration Files

```
/etc/traefik/
├── traefik.yaml        # Static configuration
├── acme.json           # Let's Encrypt certificates
└── dynamic/            # Dynamic configurations
    ├── routes.yaml
    ├── services.yaml
    └── middlewares.yaml

/etc/traefikctl/
└── config.yaml         # traefikctl configuration
```

## Static Configuration

The static configuration is loaded at Traefik startup and includes:

- Entry points
- API and dashboard
- TLS settings
- Provider configuration
- Logging

**File:** `/etc/traefik/traefik.yaml`

## Dynamic Configuration

Dynamic configurations are hot-reloaded without restarting Traefik:

- HTTP routers
- TCP routers
- Services
- Middlewares

**Directory:** `/etc/traefik/dynamic/*.yaml`

## Next Steps

- **[Static Configuration](static.md)** - Configure entrypoints, API, TLS
- **[Dynamic Configuration](dynamic.md)** - Routes, services, load balancing
- **[ACME/Let's Encrypt](acme.md)** - HTTPS certificates
