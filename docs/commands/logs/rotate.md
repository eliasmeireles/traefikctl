# logs rotate

Truncate Traefik log files in-place when they exceed a size threshold.

## Synopsis

```bash
sudo traefikctl logs rotate [flags]
```

## How It Works

When a target file exceeds `--max-size`, it is **truncated to zero bytes
without releasing the inode** — Traefik keeps writing through the same
open file descriptor, so no `SIGUSR1`, reload, or restart is needed.

If `--keep > 0`, the current contents are first copied to `file.1`,
shifting older copies to `file.2`, `file.3`, ..., dropping anything
beyond `--keep`.

## Examples

```bash
# Truncate at the default 100MB threshold
sudo traefikctl logs rotate

# Use a custom threshold
sudo traefikctl logs rotate --max-size 50M

# Keep 3 historical copies, gzipped
sudo traefikctl logs rotate --max-size 200M --keep 3 --compress

# Force rotation regardless of size
sudo traefikctl logs rotate --force

# Rotate a specific file
sudo traefikctl logs rotate --file /var/log/traefik/traefik.log
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-size` | `100M` | Threshold before rotation (e.g. `50M`, `1G`, `512K`, `1024B`) |
| `--keep` | `0` | Number of rotated copies to keep (`0` = just truncate) |
| `--compress` | `false` | Gzip rotated copies (used with `--keep > 0`) |
| `--force` | `false` | Rotate even if below threshold |
| `--file` | _(unset)_ | Override target files (default: `traefik.log` + `access.log`) |

### Size suffixes

`--max-size` accepts a number followed by an optional suffix
(case-insensitive): `B`, `K`, `M`, `G`. A bare number is interpreted as
bytes.

## Default Targets

When `--file` is not provided, both files are rotated:

- `/var/log/traefik/traefik.log`
- `/var/log/traefik/access.log`

## Output

```
[INFO] Rotating /var/log/traefik/traefik.log (size: 142.3M, threshold: 100.0M)
[INFO] Rotating /var/log/traefik/access.log (size: 118.7M, threshold: 100.0M)
```

If nothing exceeds the threshold:

```
[INFO] No files needed rotation (all below 100.0M)
```

## Why Truncate Instead of Rename?

Renaming or unlinking the active log file would orphan Traefik's open
file descriptor — bytes would still be written, but to the now-renamed
inode, leaving the new path empty. `truncate(2)` keeps the inode and
the fd valid while resetting the file's size to 0 from the kernel's
perspective, so newly written bytes start at offset 0 in the same file.

## See Also

- [`logs rotation`](rotation.md) — schedule this command automatically
