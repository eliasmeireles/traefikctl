package traefik

const DefaultStaticConfig = `# Traefik Static Configuration
# Managed by traefikctl
# Location: /etc/traefik/traefik.yaml

log:
  level: INFO
  filePath: /var/log/traefik/traefik.log
  format: common

accessLog:
  filePath: /var/log/traefik/access.log
  format: common

api:
  dashboard: true
  insecure: false

entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

tls:
  options:
    default:
      minVersion: VersionTLS12
      cipherSuites:
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
        - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
        - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
        - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305

providers:
  file:
    directory: /etc/traefik/dynamic/
    watch: true

serversTransport:
  dialTimeout: 5s
  responseHeaderTimeout: 50s
  idleConnTimeout: 50s
`

const DefaultDynamicExample = `# Example traefikctl dynamic configuration
# Place YAML files in /etc/traefik/dynamic/ and Traefik will auto-detect changes.
#
# For more examples see: https://github.com/eliasmeireles/traefikctl/tree/main/examples

# ===========================================================================
# HTTP Routers & Services
# ===========================================================================
# http:
#   routers:
#     # Simple reverse proxy by domain
#     my-app:
#       rule: "Host(` + "`" + `app.example.com` + "`" + `)"
#       entryPoints:
#         - web
#       service: my-app-svc
#
#     # Multiple domains to same service
#     my-site:
#       rule: "Host(` + "`" + `example.com` + "`" + `) || Host(` + "`" + `www.example.com` + "`" + `)"
#       entryPoints:
#         - web
#       service: my-site-svc
#
#     # Default catch-all (lowest priority)
#     default:
#       rule: "PathPrefix(` + "`" + `/` + "`" + `)"
#       entryPoints:
#         - web
#       service: default-svc
#       priority: 1
#
#   services:
#     my-app-svc:
#       loadBalancer:
#         servers:
#           - url: "http://10.8.0.2:8080"
#
#     my-site-svc:
#       loadBalancer:
#         servers:
#           - url: "http://10.8.0.3:80"
#           - url: "http://10.8.0.4:80"  # Load balanced
#
#     default-svc:
#       loadBalancer:
#         servers:
#           - url: "http://127.0.0.1:8080"

# ===========================================================================
# TCP Routers & Services (e.g., database, message queue)
# ===========================================================================
# tcp:
#   routers:
#     postgres:
#       rule: "HostSNI(` + "`" + `*` + "`" + `)"
#       entryPoints:
#         - postgres  # Must be defined in traefik.yaml entryPoints
#       service: postgres-svc
#
#   services:
#     postgres-svc:
#       loadBalancer:
#         servers:
#           - address: "10.8.0.10:5432"
`
