# resource backend

Manage backend servers for load balancing.

## Synopsis

```bash
traefikctl resource backend <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `add` | Add a backend server |
| `remove` | Remove a backend server |

## Examples

```bash
# Add backend server
traefikctl resource backend add --name myapp --address 10.0.0.3:8080

# Remove backend server
traefikctl resource backend remove --name myapp --address 10.0.0.3:8080
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Router/service name |
| `--address` | Yes | Server address (`ip:port`) |
| `--file` | No | Dynamic config file |

## Load Balancing

Adding multiple backends enables round-robin load balancing:

```bash
traefikctl resource backend add --name myapp --address 10.0.0.2:8080
traefikctl resource backend add --name myapp --address 10.0.0.3:8080
traefikctl resource backend add --name myapp --address 10.0.0.4:8080
```

Result:

```yaml
services:
  myapp-svc:
    loadBalancer:
      servers:
        - url: "http://10.0.0.2:8080"
        - url: "http://10.0.0.3:8080"
        - url: "http://10.0.0.4:8080"
```

## Health Checks

Traefik automatically health checks all backend servers:

| Behavior | Action |
|----------|--------|
| Server healthy | Traffic forwarded |
| Server unhealthy | Traffic not forwarded |
| All servers down | 503 Service Unavailable |

## Removing Backends

```bash
# Graceful removal (drain)
traefikctl resource backend remove --name myapp --address 10.0.0.2:8080
```

> Traffic to the removed server stops immediately.
