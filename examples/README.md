# traefikctl Examples

## Static Configuration

- [config.yaml](config.yaml) - traefikctl agent configuration
- [traefik.yaml](traefik.yaml) - Traefik static configuration (entrypoints, TLS, providers)

## Dynamic Configuration (Routers & Services)

- [http-simple.yaml](http-simple.yaml) - Simple HTTP reverse proxy
- [http-domain-routing.yaml](http-domain-routing.yaml) - Multi-domain routing on same port
- [http-load-balancer.yaml](http-load-balancer.yaml) - HTTP load balancer with multiple servers
- [tcp-proxy.yaml](tcp-proxy.yaml) - TCP proxy (database, message queue)
- [full-setup.yaml](full-setup.yaml) - Complete setup with HTTP + TCP services
