# resource check

Check backend server connectivity.

## Synopsis

```bash
traefikctl resource check [flags]
```

## Examples

```bash
# Check all backends
traefikctl resource check

# Check specific router
traefikctl resource check --name myapp

# Check specific file
traefikctl resource check --file /etc/traefik/dynamic/services.yaml

# Increase timeout
traefikctl resource check --timeout 10
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (all) | Check specific router |
| `--file` | (all) | Check specific file |
| `--timeout` | `3` | Connection timeout (seconds) |

## Output Example

```
Checking backend connectivity...

✓ myapp (10.0.0.2:8080) - Reachable
✓ api (10.0.0.3:3000) - Reachable
✗ database (10.0.0.10:5432) - Connection refused
⚠ postgres (10.0.0.11:6379) - Timeout (5s)

Summary: 2/4 servers reachable
```

## Status Codes

| Status | Meaning |
|--------|---------|
| `✓` | Server is reachable |
| `✗` | Connection refused or error |
| `⚠` | Timeout |

## Use Cases

### Pre-deployment Check

```bash
# Before adding new backend
traefikctl resource add --name newapp --address 10.0.0.5:8080
traefikctl resource check --name newapp
```

### Health Monitoring

```bash
# Regular health check
while true; do
  traefikctl resource check
  sleep 60
done
```

### CI/CD Integration

```bash
# Fail if any backend is down
traefikctl resource check || exit 1
```
