# check

Validate system configuration and setup.

## Synopsis

```bash
traefikctl check [flags]
```

## Examples

```bash
# Run all checks
traefikctl check
```

## What It Checks

| Check | Description |
|-------|-------------|
| Traefik Binary | Checks if `/usr/local/bin/traefik` exists |
| Traefik User | Verifies `traefik` user exists |
| Directories | Ensures required directories exist |
| Static Config | Checks `/etc/traefik/traefik.yaml` exists |
| Service | Verifies systemd service file |

## Output Example

```
Running system checks...

✓ Traefik binary installed
✓ Traefik user exists
✓ Directories exist:
    /etc/traefik
    /etc/traefik/dynamic
    /var/log/traefik
✓ Static config exists
✓ Systemd service installed

All checks passed!
```

## Troubleshooting

If checks fail:

| Check Failed | Solution |
|--------------|----------|
| Traefik binary | Run `traefikctl install` |
| Directories | Run `traefikctl install` |
| Static config | Run `traefikctl config --generate` |
| Service | Run `traefikctl service install` |
