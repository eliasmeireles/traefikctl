# resource

Manage HTTP and TCP routes.

## Synopsis

```bash
traefikctl resource <subcommand> [flags]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `add` | Add a new route |
| `list` | List all routes |
| `update` | Update an existing route |
| `remove` | Remove a route |
| `enable` | Enable a disabled route |
| `disable` | Disable a route |
| `copy` | Copy a route |
| `backend` | Manage backend servers |
| `check` | Check backend connectivity |

## Resource Types

### HTTP Routes

Routes HTTP traffic based on host and path rules.

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      entryPoints:
        - web
      service: myapp-svc
```

### TCP Routes

Routes TCP traffic for protocols like databases.

```yaml
tcp:
  routers:
    postgres:
      rule: "HostSNI(`*`)"
      entryPoints:
        - postgres
      service: postgres-svc
```

## Configuration Files

Routes are stored in `/etc/traefik/dynamic/`:

```
/etc/traefik/dynamic/
├── routes.yaml          # Default routes
├── services.yaml        # Additional services
└── middlewares.yaml     # Middleware definitions
```

## Next Steps

- **[resource add](add.md)** - Add new routes
- **[resource list](list.md)** - List all routes
- **[resource update](update.md)** - Modify routes
- **[resource remove](remove.md)** - Remove routes
- **[resource enable/disable](toggle.md)** - Toggle routes
- **[resource copy](copy.md)** - Duplicate routes
- **[resource backend](backend.md)** - Manage backends
- **[resource check](check.md)** - Health checks
