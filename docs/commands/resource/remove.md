# resource remove

Remove a route from the configuration.

## Synopsis

```bash
traefikctl resource remove [flags]
```

## Examples

```bash
# Remove by name
traefikctl resource remove --name myapp

# Remove from specific file
traefikctl resource remove --name postgres --file /etc/traefik/dynamic/tcp.yaml
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Router name to remove |
| `--file` | No | Dynamic config file |

## Safety Confirmation

```bash
$ traefikctl resource remove --name myapp

⚠️  This will remove the route "myapp"
   Address: 10.0.0.2:8080
   Domain: app.example.com

Continue? [y/N]
```

## What Gets Removed

- Router entry
- Service entry (if not used by other routers)

## Backup

Before removal, the configuration is saved to:
`/etc/traefikctl/disabled/<name>-<timestamp>.yaml`

## Restoration

To restore a removed route:

```bash
# Copy from disabled folder
cp /etc/traefikctl/disabled/myapp-*.yaml /etc/traefik/dynamic/

# Or use resource copy
traefikctl resource copy --from myapp-20240115-103045 --name myapp
```
