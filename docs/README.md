# traefikctl

> A powerful CLI tool for managing Traefik proxy configurations

[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/eliasmeireles/traefikctl)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

traefikctl simplifies the management of Traefik reverse proxy through an intuitive command-line interface. From installing Traefik to managing complex routing configurations, traefikctl provides all the tools you need.

## Features

- **Easy Installation** - Install Traefik from GitHub releases with a single command
- **Route Management** - Add, update, remove, enable/disable HTTP and TCP routes
- **Middleware Support** - Configure rate limiting, authentication, redirects, and more
- **Service Management** - Install and manage Traefik as a systemd service
- **Log Rotation** - Truncate-in-place rotation, schedulable via systemd timer or logrotate
- **HAProxy Migration** - Convert existing HAProxy configurations to Traefik
- **Let's Encrypt** - Automatic HTTPS certificate management
- **Self-Update** - Keep traefikctl up to date automatically

## Quick Demo

```bash
# Install Traefik
traefikctl install

# Generate configuration
traefikctl config --generate

# Add a new route
traefikctl resource add --name myapp --domain app.example.com --address 10.0.0.2:8080

# Install as service
traefikctl service install

# View status
traefikctl status
```

## Why traefikctl?

| Feature | Manual | traefikctl |
|---------|--------|------------|
| Install Traefik | Multiple steps | One command |
| Add Route | Edit YAML manually | Guided command |
| Enable HTTPS | Complex config | `--tls --cert-resolver` flags |
| Migrate from HAProxy | Manual rewrite | `traefikctl haproxy export` |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         traefikctl                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌──────────┐  ┌───────────┐  ┌────────────┐ │
│  │ Install │  │  Config   │  │  Service  │  │   Update   │ │
│  └─────────┘  └──────────┘  └───────────┘  └────────────┘ │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Resource Manager                   │  │
│  │  ┌──────┐  ┌──────┐  ┌──────┐  ┌───────┐  ┌───────┐  │  │
│  │  │ Add  │  │ List │  │ Update │  │ Remove │  │ Copy  │  │  │
│  │  └──────┘  └──────┘  └──────┘  └───────┘  └───────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Middleware Manager                  │  │
│  │  ┌──────┐  ┌──────┐  ┌──────┐                         │  │
│  │  │ Add  │  │ List │  │Remove │                         │  │
│  │  └──────┘  └──────┘  └──────┘                         │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Traefik                                │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐                    ┌───────────────────┐  │
│  │  Static Cfg  │                    │  Dynamic Configs  │  │
│  │  traefik.yaml│                    │  /dynamic/*.yaml   │  │
│  └──────────────┘                    └───────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

After installation, traefikctl manages the following structure:

```
/etc/traefik/
├── traefik.yaml           # Static configuration
├── acme.json              # Let's Encrypt certificates
└── dynamic/               # Hot-reloaded configurations
    └── *.yaml

/etc/traefikctl/
└── disabled/              # Disabled route snapshots

/var/log/traefik/
├── traefik.log            # Application logs
└── access.log            # Access logs

/usr/local/bin/
├── traefik               # Traefik binary
└── traefikctl            # CLI binary
```

## Getting Started

Ready to dive in? Here's how to get started:

1. **[Installation](installation.md)** - Install traefikctl on your system
2. **[Quick Start](quickstart.md)** - Learn the basics in 5 minutes
3. **[Commands Reference](commands/index.md)** - Explore all available commands
4. **[Configuration Guide](configuration/index.md)** - Deep dive into configuration

## License

MIT License - See [LICENSE](LICENSE) for details.
