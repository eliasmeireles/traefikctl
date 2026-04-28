# resource enable / resource disable

Enable or disable routes without removing them.

## Synopsis

```bash
traefikctl resource enable --name <name> [flags]
traefikctl resource disable --name <name> [flags]
```

## Examples

```bash
# Disable a route
traefikctl resource disable --name myapp

# Disable from specific file
traefikctl resource disable --name myapp --file /etc/traefik/dynamic/services.yaml

# Enable a route
traefikctl resource enable --name myapp
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Router name |
| `--file` | No | Dynamic config file (for disable) |

## How It Works

### Disable

1. Reads current configuration
2. Copies to `/etc/traefikctl/disabled/`
3. Removes from dynamic directory
4. Traefik stops routing to this service

### Enable

1. Finds matching disabled configuration
2. Copies back to dynamic directory
3. Traefik starts routing again

## Use Cases

### Maintenance

```bash
# Disable for maintenance
traefikctl resource disable --name myapp
# ... perform maintenance ...
traefikctl resource enable --name myapp
```

### Blue-Green Deployment

```bash
# Disable old version
traefikctl resource disable --name api-v1

# Add new version
traefikctl resource add --name api-v2 --domain api.example.com --address 10.0.0.5:3000

# If issues, rollback
traefikctl resource disable --name api-v2
traefikctl resource enable --name api-v1
```

### A/B Testing

```bash
# Split traffic 50/50
traefikctl resource add --name app-a --domain app.example.com --address 10.0.0.2:8080
traefikctl resource add --name app-b --domain app.example.com --address 10.0.0.3:8080
```

## Disabled Files Location

```
/etc/traefikctl/disabled/
├── myapp-20240115-103045.yaml
├── myapp-20240115-120000.yaml
└── api-20240116-090000.yaml
```
