# middleware remove

Remove an unreferenced HTTP or TCP middleware.

```bash
sudo traefikctl middleware remove --name vpn-only --protocol tcp
sudo traefikctl middleware remove --name old-auth --file /etc/traefik/dynamic/custom.yaml
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Middleware name |
| `--file` | No | Dynamic config file |
| `--protocol` | No | `http` or `tcp`; otherwise inferred from the file |

Removal fails if routers in any YAML file in the selected directory still
reference the middleware. The error lists their names. Both `vpn-only` and
`vpn-only@file` references are checked in the matching protocol namespace.
Remove those references with `resource update --middleware` before retrying;
`--middleware=` clears all middleware references on that router.

When a name exists in both HTTP and TCP, `--protocol` is required. Removal does
not detach middleware from routers and has no force option.
