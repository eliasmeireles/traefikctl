# resource list

List all configured routes.

## Synopsis

```bash
traefikctl resource list [flags]
```

## Examples

```bash
# List all routes
traefikctl resource list

# List routes from specific file
traefikctl resource list --file /etc/traefik/dynamic/services.yaml
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | (all) | List routes from specific file |

## Output Example

```
╔═══════════════════════════════════════════════════════════════════════╗
║                              HTTP Routes                               ║
╠═════════╤═══════════════════════╤═══════════════════╤═════════════════╣
║ Name    │ Domain                │ Address           │ Middlewares     ║
╠═════════╪═══════════════════════╪═══════════════════╪═════════════════╣
║ myapp   │ app.example.com       │ 10.0.0.2:8080     │ rate-limit      ║
║ api     │ api.example.com       │ 10.0.0.3:3000     │ auth            ║
║ admin   │ admin.example.com     │ 10.0.0.4:8081     │ -               ║
╚═════════╧═══════════════════════╧═══════════════════╧═════════════════╝

╔═══════════════════════════════════════════════════════════════════════╗
║                              TCP Routes                               ║
╠═════════╤═══════════════════════╤═══════════════════╤═════════════════╣
║ Name    │ Entrypoint            │ Address           │ Type            ║
╠═════════╪═══════════════════════╪═══════════════════╪═════════════════╣
║ postgres│ postgres              │ 10.0.0.10:5432    │ TCP             ║
║ redis   │ redis                 │ 10.0.0.11:6379     │ TCP             ║
╚═════════╧═══════════════════════╧═══════════════════╧═════════════════╝
```

## JSON Output

```bash
traefikctl resource list --format json
```

```json
{
  "http": [
    {
      "name": "myapp",
      "domain": "app.example.com",
      "address": "10.0.0.2:8080",
      "middlewares": ["rate-limit"]
    }
  ],
  "tcp": [
    {
      "name": "postgres",
      "entrypoint": "postgres",
      "address": "10.0.0.10:5432"
    }
  ]
}
```
