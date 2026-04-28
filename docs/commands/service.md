# service

Manage the Traefik systemd service.

## Synopsis

```bash
traefikctl service <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `install` | Install systemd service unit |
| `uninstall` | Remove systemd service unit |
| `start` | Start the service |
| `stop` | Stop the service |
| `restart` | Restart the service |
| `reload` | Hot reload configuration |
| `status` | Show service status |
| `logs` | Show service logs |

## Examples

```bash
# Install and start service
traefikctl service install
traefikctl service start

# Restart service
traefikctl service restart

# Hot reload (no restart)
traefikctl service reload

# View logs
traefikctl service logs
traefikctl service logs -f

# Check status
traefikctl service status

# Uninstall
traefikctl service uninstall
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | `traefikctl` | Service name |

## Service File

Installed at: `/etc/systemd/system/traefikctl.service`

```ini
[Unit]
Description=Traefik Proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStart=/usr/local/bin/traefik --configfile /etc/traefik/traefik.yaml
Restart=on-failure
RestartSec=10s
User=traefik

[Install]
WantedBy=multi-user.target
```

## Managing Service

```bash
# Start on boot
sudo systemctl enable traefikctl

# Disable start on boot
sudo systemctl disable traefikctl

# View detailed status
sudo systemctl status traefikctl

# View logs with journal
sudo journalctl -u traefikctl -f
```
