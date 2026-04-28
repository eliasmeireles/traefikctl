# Commands Overview

Complete reference of all traefikctl commands.

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `/etc/traefikctl/config.yaml` | Config file path |

## Command Tree

```
traefikctl
├── install          Install or upgrade Traefik binary
├── update           Update traefikctl (the CLI itself)
├── config           Generate/view configuration
├── service          Manage systemd service
├── status           Show service status
├── logs             View logs
│   ├── rotate       Truncate logs above a size threshold
│   └── rotation     Install/uninstall automatic rotation
├── check            Validate system setup
├── resource         Manage routes
│   ├── add          Add new route
│   ├── list         List all routes
│   ├── update       Update route
│   ├── remove       Remove route
│   ├── enable       Enable route
│   ├── disable      Disable route
│   ├── copy         Copy route
│   ├── backend      Manage backend servers
│   └── check        Check backend connectivity
├── middleware        Manage middlewares
│   ├── add          Add middleware
│   ├── list         List middlewares
│   └── remove       Remove middleware
└── haproxy          HAProxy utilities
    └── export       Convert HAProxy config
```

## Command Index

### Installation & Updates

| Command | Description |
|---------|-------------|
| [install](install.md) | Install or upgrade the Traefik binary |
| [update](update.md) | Update the traefikctl CLI itself |

### Configuration

| Command | Description |
|---------|-------------|
| [config](config.md) | Generate/view configurations |
| [check](check.md) | Validate system setup |

### Service Management

| Command | Description |
|---------|-------------|
| [service](service.md) | Manage systemd service |
| [status](status.md) | Show service status |
| [logs](logs.md) | View logs |
| [logs rotate](logs/rotate.md) | Truncate logs above a size threshold |
| [logs rotation](logs/rotation.md) | Install automatic rotation (timer or logrotate) |

### Route Management

| Command | Description |
|---------|-------------|
| [resource add](resource/add.md) | Add new route |
| [resource list](resource/list.md) | List all routes |
| [resource update](resource/update.md) | Update route |
| [resource remove](resource/remove.md) | Remove route |
| [resource enable](resource/toggle.md) | Enable route |
| [resource disable](resource/toggle.md) | Disable route |
| [resource copy](resource/copy.md) | Copy route |
| [resource backend](resource/backend.md) | Manage backends |
| [resource check](resource/check.md) | Check backends |

### Middleware

| Command | Description |
|---------|-------------|
| [middleware add](middleware/add.md) | Add middleware |
| [middleware list](middleware/list.md) | List middlewares |
| [middleware remove](middleware/remove.md) | Remove middleware |

### Utilities

| Command | Description |
|---------|-------------|
| [haproxy export](haproxy.md) | Convert HAProxy config |

## Usage Examples

### Help

```bash
traefikctl --help
traefikctl <command> --help
```

### Tab Completion

```bash
traefikctl <tab><tab>
traefikctl resource <tab><tab>
```

### Config File

```bash
traefikctl --config /path/to/config.yaml <command>
```
