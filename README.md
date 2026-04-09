# traefikctl

[![CI](https://github.com/eliasmeireles/traefikctl/actions/workflows/ci.yml/badge.svg)](https://github.com/eliasmeireles/traefikctl/actions/workflows/ci.yml)
[![Release](https://github.com/eliasmeireles/traefikctl/actions/workflows/release.yml/badge.svg)](https://github.com/eliasmeireles/traefikctl/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/eliasmeireles/traefikctl)](https://goreportcard.com/report/github.com/eliasmeireles/traefikctl)

Traefik Control — A CLI tool for managing Traefik proxy configurations.

## Features

- **Binary Installation**: Download and install Traefik from GitHub releases
- **Config Management**: Generate, view, and extend static + dynamic configurations
- **Systemd Service**: Install, start, stop, restart, reload, and tail logs
- **Route Management**: Add, list, check, copy, enable, disable HTTP and TCP routes
- **Backend Management**: Add or remove upstream servers for load balancing
- **Middleware Management**: Add, list, and remove middlewares (redirect-https, rate-limit, basic-auth, strip-prefix)
- **HTTPS / Let's Encrypt**: One-command TLS and ACME configuration
- **Status Dashboard**: Service state, Traefik version, and route count at a glance
- **Self-Update**: Download and install the latest traefikctl release

## Installation

### Build from Source

```bash
make build
sudo make install
```

### Quick install script

```bash
curl -fsSL https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash
```

### Install Traefik binary

```bash
sudo traefikctl install
sudo traefikctl install --version v3.3.5   # specific version
traefikctl install --check                 # verify installed version
```

## Quick Start

```bash
# 1. Install Traefik
sudo traefikctl install

# 2. Generate initial configs
sudo traefikctl config --generate

# 3. Install and start the systemd service
sudo traefikctl service install
sudo systemctl start traefikctl

# 4. Add your first route
sudo traefikctl resource add --name my-app --domain app.example.com --address 10.0.0.2:8080

# 5. Check status
traefikctl status
```

## Commands

### version

```bash
traefikctl version
traefikctl --version
```

### status

Show service state, Traefik version, and a summary of all configured routes.

```bash
traefikctl status
```

### logs

Tail Traefik logs via journalctl or log file.

```bash
traefikctl logs                  # follow mode (default)
traefikctl logs --follow=false   # print last lines and exit
traefikctl logs -n 100           # show last 100 lines
```

### update

Self-update traefikctl to the latest release from GitHub.

```bash
sudo traefikctl update                    # latest release
sudo traefikctl update --version v0.1.0  # specific version
```

---

### config

```bash
# Generate static + dynamic config files
sudo traefikctl config --generate

# Overwrite existing configs
sudo traefikctl config --generate --force

# View all configs (with comments)
traefikctl config --view

# View only active config (without comments)
traefikctl config --view --clean

# Append Let's Encrypt ACME config
sudo traefikctl config --acme --acme-email you@example.com
```

---

### service

```bash
sudo traefikctl service install     # install systemd unit
sudo traefikctl service uninstall   # remove systemd unit
traefikctl service status           # check service state
traefikctl service logs             # tail service logs
sudo traefikctl service restart     # full restart
sudo traefikctl service reload      # hot reload (no downtime)
```

---

### resource add

Add an HTTP or TCP route to the dynamic config.

```bash
# Basic HTTP route
sudo traefikctl resource add --name my-app --domain app.example.com --address 10.0.0.2:8080

# HTTPS with Let's Encrypt
sudo traefikctl resource add --name my-app --domain app.example.com --address 10.0.0.2:8080 \
  --redirect-https --tls --cert-resolver letsencrypt

# Attach an existing middleware
sudo traefikctl resource add --name my-api --domain api.example.com --address 10.0.0.3:3000 \
  --entrypoint websecure --middleware my-rate-limit

# TCP route
sudo traefikctl resource add --name postgres --address 10.0.0.10:5432 --type tcp --entrypoint postgres
```

| Flag | Default | Description |
|---|---|---|
| `--name` | required | Router and service name |
| `--address` | required | Backend address (`ip:port`) |
| `--domain` | — | Host rule (`Host(\`...\`)`) |
| `--entrypoint` | `web` | Traefik entrypoint |
| `--type` | `http` | `http` or `tcp` |
| `--middleware` | — | Attach middleware(s) by name (repeatable) |
| `--redirect-https` | false | Auto HTTP→HTTPS redirect |
| `--tls` | false | Enable TLS on the router |
| `--cert-resolver` | — | Let's Encrypt resolver name |

### resource list

List all configured HTTP and TCP routes across all dynamic config files.

```bash
traefikctl resource list
```

### resource check

Test backend reachability for all routes.

```bash
traefikctl resource check
traefikctl resource check --timeout 5s
```

### resource enable / disable

Toggle a route on or off without deleting it.

```bash
sudo traefikctl resource disable --name my-app
sudo traefikctl resource enable  --name my-app
```

Disabled configs are stored in `/etc/traefikctl/disabled/`.

### resource copy

Clone an existing route to a new name and/or domain.

```bash
sudo traefikctl resource copy --from my-app --name my-app-staging --domain staging.example.com
```

### resource backend add / remove

Add or remove upstream servers from a service's load balancer pool.

```bash
sudo traefikctl resource backend add    --name my-app --address 10.0.0.3:8080
sudo traefikctl resource backend remove --name my-app --address 10.0.0.3:8080
```

---

### middleware add

```bash
# HTTP → HTTPS redirect
sudo traefikctl middleware add --name redirect-to-https --type redirect-https

# Rate limit (10 req/s, burst 20)
sudo traefikctl middleware add --name api-limit --type rate-limit \
  --opt average=10 --opt burst=20

# Basic auth
sudo traefikctl middleware add --name admin-auth --type basic-auth \
  --opt users=admin:$$apr1$$...

# Strip prefix
sudo traefikctl middleware add --name strip-api --type strip-prefix \
  --opt prefix=/api
```

### middleware list

```bash
traefikctl middleware list
```

### middleware remove

```bash
sudo traefikctl middleware remove --name api-limit
```

---

## Configuration Files

### Static Config (`/etc/traefik/traefik.yaml`)

Controls entrypoints, TLS, providers, and logging. Generated by `traefikctl config --generate`.

### Dynamic Config (`/etc/traefik/dynamic/*.yaml`)

Defines routers, services, and middlewares. Traefik watches this directory and applies changes automatically — no restart needed.

## Directory Structure

```
/etc/traefik/
├── traefik.yaml           # Static config
├── acme.json              # Let's Encrypt certificates
└── dynamic/               # Hot-reloaded configs
    └── *.yaml

/etc/traefikctl/
└── disabled/              # Disabled route snapshots

/var/log/traefik/
├── traefik.log
└── access.log

/usr/local/bin/
├── traefik                # Traefik binary
└── traefikctl             # This CLI
```
