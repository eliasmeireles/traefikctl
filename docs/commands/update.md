# update

Update the `traefikctl` CLI itself to the latest **stable** release, or
to a specific version.

## Synopsis

```bash
sudo traefikctl update [flags]
```

## Stable-only by default

When `--version` is **not** provided, `update` only considers releases
whose tag matches `^v\d+\.\d+\.\d+$` (e.g. `v0.0.4`, `v1.2.3`) and that
are not marked as draft or pre-release on GitHub. Pre-release tags such
as `v0.0.4-rc-05` are intentionally skipped — to install one, pass
`--version` explicitly.

## Examples

```bash
# Update to the latest stable release
sudo traefikctl update

# Pin a specific stable version
sudo traefikctl update --version v0.0.4

# Install a release candidate (opt-in)
sudo traefikctl update --version v0.0.4-rc-05
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | _(latest stable)_ | Specific version to install. Bypasses the stable-only filter. |

## What It Does

1. Lists releases from the GitHub API and picks the most recent stable
   tag (or uses `--version` verbatim when set)
2. Downloads the matching binary asset
3. Atomically replaces `/usr/local/bin/traefikctl` (with an EXDEV
   fallback when `/tmp` and the install dir live on different
   filesystems)
4. Preserves existing configurations

## Output

```
✓ Checking for updates...
✓ Downloading traefikctl v0.2.0...
✓ Updating /usr/local/bin/traefikctl...
✓ Update complete! Version: v0.2.0
```

## Rollback

If you need to rollback:

```bash
# Reinstall specific version
traefikctl update --version v0.1.0
```
