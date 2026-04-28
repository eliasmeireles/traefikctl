# middleware list

List all configured middlewares.

## Synopsis

```bash
traefikctl middleware list [flags]
```

## Examples

```bash
# List all middlewares
traefikctl middleware list

# List from specific file
traefikctl middleware list --file /etc/traefik/dynamic/middlewares.yaml
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | (all) | List from specific file |

## Output Example

```
╔═══════════════════════════════════════════════════════════════════════╗
║                            Middlewares                                 ║
╠═══════════════════╤═══════════════════╤═════════════════════════════════╣
║ Name              │ Type             │ Configuration                  ║
╠═══════════════════╪═══════════════════╪═════════════════════════════════╣
║ redirect-to-https │ redirectScheme   │ scheme=https, permanent=true    ║
║ rate-limit        │ rateLimit        │ average=100, burst=50           ║
║ admin-auth        │ basicAuth        │ users=1                         ║
║ strip-api         │ stripPrefix      │ prefixes=/api                   ║
╚═══════════════════╧═══════════════════╧═════════════════════════════════╝
```

## JSON Output

```bash
traefikctl middleware list --format json
```

```json
{
  "middlewares": [
    {
      "name": "redirect-to-https",
      "type": "redirectScheme",
      "config": {
        "scheme": "https",
        "permanent": true
      }
    },
    {
      "name": "rate-limit",
      "type": "rateLimit",
      "config": {
        "average": 100,
        "burst": 50
      }
    }
  ]
}
```
