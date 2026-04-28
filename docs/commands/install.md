# install

Install or upgrade the Traefik binary from GitHub releases.

## Synopsis

Downloads the Traefik binary, creates the necessary system user and
directories, and sets up permissions for low port binding. The same
command upgrades an existing installation when `--upgrade` is set.

```bash
sudo traefikctl install [flags]
```

## Examples

```bash
# Install the default pinned version (when not yet installed)
sudo traefikctl install

# Install a specific version
sudo traefikctl install --version v3.4.0

# Install the most recent release (resolved from the Traefik GitHub API)
sudo traefikctl install --version latest

# Upgrade an existing installation to a specific version
sudo traefikctl install --upgrade --version v3.4.0

# Upgrade to whatever the latest release is
sudo traefikctl install --upgrade --version latest

# Only check whether Traefik is installed
traefikctl install --check
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | _(pinned default)_ | Traefik version to install (e.g. `v3.4.0` or `latest`) |
| `--upgrade` | `false` | Replace the existing Traefik binary with the requested version |
| `--check` | `false` | Only check if Traefik is installed (does not download) |

`--version latest` queries
`https://api.github.com/repos/traefik/traefik/releases/latest` and uses
the `tag_name` it returns.

## Behavior

| State | Without `--upgrade` | With `--upgrade` |
|-------|---------------------|------------------|
| Not installed | Downloads and installs the requested version | Same — installs the requested version |
| Already installed | Skips download, prints current version, runs setup steps (user/dirs) | Downloads the requested version and replaces the binary |

The traefik service may already be running when you upgrade. Linux
keeps the running binary alive via the open inode, so replacing the
file is safe — but the new version is only loaded after a restart:

```bash
sudo traefikctl service restart
```

## What It Does

1. Resolves the requested version (expanding `latest` if needed)
2. Downloads the Traefik tarball for the host architecture (`amd64` or `arm64`)
3. Extracts and installs the binary to `/usr/local/bin/traefik`
4. Sets `cap_net_bind_service=+ep` so it can bind ports < 1024 without root
5. Creates the `traefik` system user and group (if missing)
6. Creates the required directories (idempotent):
   - `/etc/traefik`
   - `/etc/traefik/dynamic`
   - `/var/log/traefik`

## Output

Fresh install:

```
[INFO] Installing Traefik v3.4.0...
[INFO] Downloading Traefik v3.4.0 for linux/amd64...
[INFO] Traefik installed at /usr/local/bin/traefik
[INFO] Installation completed:
Version:      3.4.0
...
```

Upgrade:

```
[INFO] Upgrading to Traefik v3.4.0...
[INFO] Downloading Traefik v3.4.0 for linux/amd64...
[INFO] Traefik installed at /usr/local/bin/traefik
[INFO] Installation completed:
Version:      3.4.0
...
[INFO] Restart Traefik to load the new binary: sudo traefikctl service restart
```

## Verification

```bash
traefikctl install --check
# or
traefikctl check
# or
traefik version
```

## See Also

- [`update`](update.md) — update the `traefikctl` CLI itself (not the Traefik binary)
- [`service`](service.md) — restart the Traefik service after an upgrade
