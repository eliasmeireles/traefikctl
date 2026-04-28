# Dynamic Configuration

Routes, services, and middlewares that can be reloaded without restart.

## Location

`/etc/traefik/dynamic/*.yaml`

## HTTP Configuration

### Basic Router and Service

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      entryPoints:
        - web
      service: myapp-svc
      
  services:
    myapp-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
```

### Multiple Backends (Load Balancing)

```yaml
http:
  services:
    myapp-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
          - url: "http://10.0.0.3:8080"
          - url: "http://10.0.0.4:8080"
```

### Router with TLS

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      entryPoints:
        - websecure
      service: myapp-svc
      tls:
        certResolver: letsencrypt
```

### Router with Middleware

```yaml
http:
  routers:
    myapp:
      rule: "Host(`app.example.com`)"
      entryPoints:
        - websecure
      service: myapp-svc
      middlewares:
        - rate-limit
        - auth
      
  middlewares:
    rate-limit:
      rateLimit:
        average: 100
        burst: 50
        
    auth:
      basicAuth:
        users:
          - "admin:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/"
```

### Path-Based Routing

```yaml
http:
  routers:
    api:
      rule: "Host(`api.example.com`) && PathPrefix(`/api`)"
      entryPoints:
        - web
      service: api-svc
      
    dashboard:
      rule: "Host(`api.example.com`) && PathPrefix(`/dashboard`)"
      entryPoints:
        - web
      service: dashboard-svc
```

## TCP Configuration

### Basic TCP Router

```yaml
tcp:
  routers:
    postgres:
      rule: "HostSNI(`*`)"
      entryPoints:
        - postgres
      service: postgres-svc
      
  services:
    postgres-svc:
      loadBalancer:
        servers:
          - address: "10.0.0.10:5432"
```

### TCP Router with TLS Termination

```yaml
tcp:
  routers:
    postgres-tls:
      rule: "HostSNI(`postgres.example.com`)"
      entryPoints:
        - postgres
      service: postgres-svc
      tls: {}
```

## Service Types

### Load Balancer

```yaml
services:
  myapp-svc:
    loadBalancer:
      servers:
        - url: "http://10.0.0.2:8080"
```

### Weighted Round Robin

```yaml
services:
  myapp-svc:
    weighted:
      services:
        - name: myapp-v1
          weight: 90
        - name: myapp-v2
          weight: 10
```

## Middleware Reference

| Middleware | Type | Purpose |
|------------|------|---------|
| Rate Limit | `rateLimit` | Limit request rate |
| Basic Auth | `basicAuth` | User/password protection |
| Digest Auth | `digestAuth` | Digest authentication |
| Forward Auth | `forwardAuth` | External authentication |
| IP Whitelist | `ipWhiteList` | IP-based access control |
| Strip Prefix | `stripPrefix` | Remove path prefix |
| Add Prefix | `addPrefix` | Add path prefix |
| Replace Path | `replacePath` | Rewrite path |
| Redirect | `redirectRegex` | Regex-based redirect |
| Circuit Breaker | `circuitBreaker` | Fail fast on errors |
| Retry | `retry` | Automatic retry |
| Timeout | `timeout` | Request timeout |
| Compress | `compress` | Gzip compression |
| Headers | `headers` | Custom headers |

## Rule Syntax

| Traefik Rule | HAProxy Equivalent |
|--------------|-------------------|
| `Host(\`example.com\`)` | `hdr(host) -i example.com` |
| `Host(\`*.example.com\`)` | Wildcard subdomain |
| `PathPrefix(\`/api\`)` | `path_beg /api` |
| `Path(\`/exact\`)` | `path /exact` |
| `Method(\`GET\`)` | `http_request method GET` |
| `Header(\`X-Api-Key\`, \`secret\`)` | `hdr(X-Api-Key) -i secret` |
| `Query(\`foo=bar\`)` | `query string match` |
