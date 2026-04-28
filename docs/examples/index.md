# Common Use Cases

Real-world examples using traefikctl.

## Simple HTTP Route

```bash
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080
```

## HTTPS with Auto Certificate

```bash
# Setup
traefikctl config --generate --acme --acme-email admin@example.com
traefikctl service restart

# Add route
traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --entrypoint websecure \
  --tls \
  --cert-resolver letsencrypt
```

## HTTP to HTTPS Redirect

```bash
# Create redirect middleware
traefikctl middleware add --name redirect-https --type redirect-https

# Apply to HTTP route
traefikctl resource update --name myapp --middleware redirect-https
```

## API with Rate Limiting

```bash
# Create middleware
traefikctl middleware add \
  --name api-limit \
  --type rate-limit \
  --opt average=100 \
  --opt burst=50

# Add route with middleware
traefikctl resource add \
  --name api \
  --domain api.example.com \
  --address 10.0.0.3:3000 \
  --middleware api-limit
```

## Password Protected Route

```bash
# Generate password hash
htpasswd -nb admin password123

# Create middleware
traefikctl middleware add \
  --name admin-auth \
  --type basic-auth \
  --opt users="admin:$$apr1$$H6uskkkW$$IgXLP6ewTrSuBkTrqE8wj/"

# Apply to route
traefikctl resource update --name admin --middleware admin-auth
```

## Multiple Backends (Load Balancer)

```bash
# Add first backend
traefikctl resource add \
  --name webapp \
  --domain web.example.com \
  --address 10.0.0.2:8080

# Add more backends
traefikctl resource backend add --name webapp --address 10.0.0.3:8080
traefikctl resource backend add --name webapp --address 10.0.0.4:8080
```

## Path-Based Routing

```yaml
# Manual: Edit dynamic config
http:
  routers:
    api:
      rule: "Host(`example.com`) && PathPrefix(`/api`)"
      service: api-svc
      
    dashboard:
      rule: "Host(`example.com`) && PathPrefix(`/dashboard`)"
      service: dashboard-svc
      
  services:
    api-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.3:3000"
    dashboard-svc:
      loadBalancer:
        servers:
          - url: "http://10.0.0.4:3001"
```

## Blue-Green Deployment

```bash
# v1 is live
traefikctl resource add \
  --name api \
  --domain api.example.com \
  --address 10.0.0.2:8080

# Deploy v2
traefikctl resource add \
  --name api-v2 \
  --domain api.example.com \
  --address 10.0.0.3:8080

# Test v2
curl -H "Host: api.example.com" http://10.0.0.3:8080/health

# If OK, disable v1
traefikctl resource disable --name api

# If issues, rollback
traefikctl resource enable --name api
traefikctl resource disable --name api-v2
```

## Canary Deployment (Weighted)

```yaml
# Manual: Create weighted service
http:
  services:
    api-weighted:
      weighted:
        services:
          - name: api-v1
            weight: 80
          - name: api-v2
            weight: 20
            
    api-v1:
      loadBalancer:
        servers:
          - url: "http://10.0.0.2:8080"
          
    api-v2:
      loadBalancer:
        servers:
          - url: "http://10.0.0.3:8080"
```

## TCP Route (PostgreSQL)

```bash
# Create TCP entrypoint (edit /etc/traefik/traefik.yaml)
# entryPoints:
#   postgres:
#     address: ":5432"

# Add TCP route
traefikctl resource add \
  --name postgres \
  --address 10.0.0.10:5432 \
  --type tcp \
  --entrypoint postgres
```

## Strip Prefix

```bash
# App runs at /api but exposed at /
traefikctl middleware add \
  --name strip-api \
  --type strip-prefix \
  --opt prefixes=/api

traefikctl resource add \
  --name myapp \
  --domain app.example.com \
  --address 10.0.0.2:8080 \
  --middleware strip-api
```

## IP Whitelist

```bash
traefikctl middleware add \
  --name internal-only \
  --type ip-whitelist \
  --opt sourcerange=192.168.1.0/24,10.0.0.0/8

traefikctl resource update --name internal-api --middleware internal-only
```

## Security Headers

```bash
traefikctl middleware add \
  --name security \
  --type headers \
  --opt sslredirect=true \
  --opt stsseconds=31536000 \
  --opt contenttypenosniff=true \
  --opt browserxssfilter=true \
  --opt forceSTSHeader=true \
  --opt stsincludeSubdomains=true \
  --opt stspreload=true

traefikctl resource update --name myapp --middleware security
```
