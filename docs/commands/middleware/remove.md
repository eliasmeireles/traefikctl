# middleware remove

Remove a middleware.

## Synopsis

```bash
traefikctl middleware remove [flags]
```

## Examples

```bash
# Remove by name
traefikctl middleware remove --name rate-limit

# Remove from specific file
traefikctl middleware remove --name old-auth --file /etc/traefik/dynamic/custom.yaml
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Middleware name |
| `--file` | No | Dynamic config file |

## Safety Check

Before removal, traefikctl checks if the middleware is in use:

```bash
$ traefikctl middleware remove --name rate-limit

⚠️  Warning: This middleware is used by:
   - myapp
   - api
   
Remove anyway? [y/N]
```

## Cascade Effects

Removing a middleware affects all routes using it:

| Route | Effect |
|-------|--------|
| Routes with single middleware | Middleware list becomes empty |
| Routes with multiple middlewares | Other middlewares remain |

After removal, manually update affected routes if needed.
