# logs rotation

Manage automatic Traefik log rotation. Two modes are supported: a
self-contained **systemd timer** (default) or a drop-in for the
system's **logrotate** daemon.

## Synopsis

```bash
sudo traefikctl logs rotation install [flags]
sudo traefikctl logs rotation uninstall
traefikctl logs rotation status
```

## Modes

| Mode | Dependency | How it runs |
|------|------------|-------------|
| `timer` (default) | None — uses systemd | A systemd timer fires `traefikctl logs rotate` on a fixed interval |
| `logrotate` | `logrotate` package | An `/etc/logrotate.d/traefikctl` drop-in, executed by the system's logrotate cron/timer (typically hourly) |

Both modes use the **truncate-in-place** strategy (see
[`logs rotate`](rotate.md#why-truncate-instead-of-rename)) — `logrotate`
mode uses `copytruncate` for the same reason.

## Subcommands

### install

Installs and enables automatic rotation.

```bash
# Default: systemd timer, every 5 minutes, truncate at 100MB
sudo traefikctl logs rotation install

# Timer with custom interval and history
sudo traefikctl logs rotation install \
  --interval 1h --max-size 200M --keep 5 --compress

# Use logrotate instead of the systemd timer
sudo traefikctl logs rotation install --mode logrotate \
  --max-size 100M --keep 3 --compress
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `timer` | Rotation mode: `timer` or `logrotate` |
| `--max-size` | `100M` | Size threshold for rotation |
| `--keep` | `0` | Number of rotated copies to keep (`0` = just truncate) |
| `--compress` | `false` | Gzip rotated copies (used with `--keep > 0`) |
| `--interval` | `5min` | Timer interval (`OnUnitActiveSec`) — `timer` mode only |
| `--binary` | `/usr/local/bin/traefikctl` | Absolute path to the `traefikctl` binary the timer should call |

### uninstall

Removes timer units, the logrotate drop-in, and reloads systemd.
Idempotent — safe to run when nothing is installed.

```bash
sudo traefikctl logs rotation uninstall
```

### status

Reports which mode (if any) is currently installed and shows the next
firing time when the timer is active.

```bash
traefikctl logs rotation status
```

Example output:

```
Timer mode:      INSTALLED (/etc/systemd/system/traefikctl-logs-rotate.timer, /etc/systemd/system/traefikctl-logs-rotate.service)
Logrotate mode:  not installed

NEXT                        LEFT          LAST                        PASSED       UNIT                          ACTIVATES
Mon 2026-04-27 20:35:00 -03 4min 12s left Mon 2026-04-27 20:30:00 -03 47s ago      traefikctl-logs-rotate.timer  traefikctl-logs-rotate.service
```

## What Gets Installed

### Timer mode

Two systemd units are written and `daemon-reload`ed, then the timer is
enabled and started:

| Path | Purpose |
|------|---------|
| `/etc/systemd/system/traefikctl-logs-rotate.service` | Oneshot unit that runs `traefikctl logs rotate ...` |
| `/etc/systemd/system/traefikctl-logs-rotate.timer` | Schedules the service at the configured interval (`OnBootSec=2min`, `Persistent=true`) |

Inspect with:

```bash
systemctl list-timers traefikctl-logs-rotate.timer
sudo systemctl start traefikctl-logs-rotate.service     # run once now
journalctl -u traefikctl-logs-rotate.service
```

### Logrotate mode

A single config file is written:

| Path | Contents |
|------|----------|
| `/etc/logrotate.d/traefikctl` | `size`, `rotate`, `copytruncate`, `notifempty`, `missingok`, `su traefik traefik` |

Test and force-run with:

```bash
sudo logrotate -d /etc/logrotate.d/traefikctl   # dry-run
sudo logrotate -f /etc/logrotate.d/traefikctl   # force now
```

`logrotate` mode requires the `logrotate` binary on `PATH` — install it
first (`sudo apt install logrotate`) or use `--mode=timer`.

## Choosing a Mode

- **Use `timer`** when you want a self-contained, predictable interval
  with no external dependency. Fine-grained intervals (`5min`, `1h`)
  are supported.
- **Use `logrotate`** when you already manage other logs through
  `logrotate` and prefer a single, consistent rotation pipeline.
  Cadence is whatever your distro schedules (typically hourly).

## See Also

- [`logs rotate`](rotate.md) — the underlying rotation command
- [`logs`](../logs.md) — viewing log output
