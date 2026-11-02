# middleware list

List HTTP and TCP middlewares, grouped by file and labeled with their protocol.

```bash
traefikctl middleware list
traefikctl middleware list --protocol tcp
traefikctl middleware list --file /etc/traefik/dynamic/middlewares.yaml
```

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | All dynamic files | Select one file |
| `--protocol` | Both | Filter by `http` or `tcp` |

The output includes the middleware name, type and configuration summary, including
source ranges for IP allowlists.
