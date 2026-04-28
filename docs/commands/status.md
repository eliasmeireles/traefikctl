# status

Show Traefik service status and configuration summary.

## Synopsis

```bash
traefikctl status [flags]
```

## Examples

```bash
# Show status
traefikctl status

# Custom service name
traefikctl status --name my-traefik
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | `traefikctl` | Systemd service name |

## Output Example

```
Service: traefikctl
Status: running (active)
Traefik Version: v3.3.5
Uptime: 2 hours 15 minutes

Routes Summary:
  HTTP Routers: 5
  TCP Routers: 1
  Middlewares: 3

Configuration:
  Static Config: /etc/traefik/traefik.yaml
  Dynamic Configs: /etc/traefik/dynamic/
  Log Level: INFO
```
