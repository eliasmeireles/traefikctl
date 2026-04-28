# logs

View Traefik logs and manage log rotation.

## Synopsis

```bash
traefikctl logs [flags]
traefikctl logs <subcommand> [flags]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [`logs rotate`](logs/rotate.md) | Truncate logs in-place when they exceed a size threshold |
| [`logs rotation`](logs/rotation.md) | Install/uninstall/inspect automatic rotation (timer or logrotate) |

## Examples

```bash
# Follow traefik log
traefikctl logs

# Print and exit (no follow)
traefikctl logs --follow=false

# Last 100 lines
traefikctl logs -n 100

# View access log
traefikctl logs --access

# From systemd journal
traefikctl logs --service
traefikctl logs --service -f

# Specific service name
traefikctl logs --service --name custom-service
```

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--follow` | `-f` | `true` | Follow log output |
| `--lines` | `-n` | `50` | Number of lines to show |
| `--access` | | `false` | Show access log instead |
| `--service` | | `false` | Read from systemd journal |
| `--name` | | `traefikctl` | Service name |

## Log Locations

| Log Type | Location |
|----------|----------|
| Traefik Log | `/var/log/traefik/traefik.log` |
| Access Log | `/var/log/traefik/access.log` |
| Journal | `journalctl -u traefikctl` |

## Log Format

Standard Traefik log format:

```
2024-01-15T10:30:45Z level="INFO" msg="Server started" protocol="http"
```

Access log format (Combined):

```
127.0.0.1 - - [15/Jan/2024:10:30:45 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com"
```

## See Also

- [`logs rotate`](logs/rotate.md) — manual, on-demand rotation
- [`logs rotation`](logs/rotation.md) — automatic rotation setup
