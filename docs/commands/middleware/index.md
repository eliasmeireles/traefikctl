# middleware

Manage Traefik middlewares.

## Synopsis

```bash
traefikctl middleware <subcommand> [flags]
```

## What are Middlewares?

Middlewares are processing steps that can modify requests/responses before they reach your backend.

## Available Middlewares

| Type | Description |
|------|-------------|
| `redirect-https` | HTTP to HTTPS redirect |
| `rate-limit` | Rate limiting |
| `basic-auth` | Basic authentication |
| `strip-prefix` | Remove path prefixes |
| `add-prefix` | Add path prefix |
| `ip-whitelist` | IP allowlist |
| `headers` | Custom headers |

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `add` | Add a new middleware |
| `list` | List all middlewares |
| `remove` | Remove a middleware |

## Next Steps

- **[middleware add](add.md)** - Create new middleware
- **[middleware list](list.md)** - View configured middlewares
- **[middleware remove](remove.md)** - Remove middleware
