# Troubleshooting

Solutions to common issues with traefikctl.

## Installation Issues

### Binary not found after install

```bash
# Verify installation
which traefikctl
ls -la /usr/local/bin/traefikctl

# If not found, reinstall
sudo curl -fsSL https://raw.githubusercontent.com/eliasmeireles/traefikctl/main/install.sh | bash
```

### Permission denied during install

Always use sudo:
```bash
sudo traefikctl install
```

### Wrong architecture

Ensure you download the correct binary:
```bash
# Check architecture
uname -m

# x86_64 = amd64
# aarch64 = arm64
```

## Configuration Issues

### Config file not found

```bash
# Generate default config
traefikctl config --generate
```

### Invalid YAML syntax

```bash
# Validate YAML
python3 -c "import yaml; yaml.safe_load(open('/etc/traefik/traefik.yaml'))"
```

### Changes not taking effect

Traefik should auto-reload. If not:
```bash
traefikctl service reload
# or
traefikctl service restart
```

## Service Issues

### Service won't start

```bash
# Check status
sudo systemctl status traefikctl

# View logs
sudo journalctl -u traefikctl -n 50

# Common fixes
sudo chown -R traefik:traefik /etc/traefik
sudo chmod +x /usr/local/bin/traefik
```

### Service fails on restart

Check config syntax:
```bash
/usr/local/bin/traefik validate --configfile /etc/traefik/traefik.yaml
```

### Port already in use

```bash
# Find what's using port 80/443
sudo netstat -tlnp | grep -E ':(80|443)'
sudo lsof -i :80
```

## Route Issues

### Route not working

1. **Verify route exists:**
```bash
traefikctl resource list
```

2. **Check backend connectivity:**
```bash
traefikctl resource check --name myapp
```

3. **Test locally:**
```bash
curl -v -H "Host: app.example.com" http://localhost:80/health
```

### 502 Bad Gateway

Backend server unreachable:

```bash
# Check if backend is up
curl http://10.0.0.2:8080/health

# Check route address
traefikctl resource list
```

### 503 Service Unavailable

All backend servers are failing health checks:

```bash
# Check backend status
traefikctl resource check --name myapp

# Manually test
curl http://10.0.0.2:8080
```

### 404 Not Found

Wrong rule configuration:

```bash
# Check rule in config
grep -A5 "myapp:" /etc/traefik/dynamic/*.yaml
```

### 502 with multiple backends

One unhealthy server affecting all:

```bash
# List all backends
traefikctl resource check --name myapp

# Remove unhealthy backend
traefikctl resource backend remove --name myapp --address 10.0.0.3:8080
```

## TLS/HTTPS Issues

### Certificate not issued

```bash
# Enable debug logging
# Edit /etc/traefik/traefik.yaml
log:
  level: DEBUG

# Restart
traefikctl service restart

# Monitor logs
traefikctl logs | grep -E "acme|challenge|certificate"
```

### HTTP challenge fails

Check firewall:
```bash
# Allow port 80
sudo ufw allow 80
sudo firewall-cmd --add-port=80/tcp
```

Verify challenge URL:
```bash
curl -I http://app.example.com/.well-known/acme-challenge/test
```

### HTTPS shows wrong certificate

Clear certificate cache:
```bash
sudo rm /etc/traefik/acme.json
sudo touch /etc/traefik/acme.json
sudo chmod 600 /etc/traefik/acme.json
traefikctl service restart
```

### Mixed content errors

All resources must use HTTPS:
```html
<!-- Bad -->
<img src="http://example.com/image.png">

<!-- Good -->
<img src="https://example.com/image.png">
```

## Middleware Issues

### Middleware not applied

1. Verify middleware exists:
```bash
traefikctl middleware list
```

2. Check middleware is attached:
```bash
grep -B5 -A10 "middlewares:" /etc/traefik/dynamic/*.yaml
```

### Rate limiting too strict

Adjust values:
```bash
traefikctl middleware update --name mylimit --type rate-limit \
  --opt average=1000 --opt burst=200
```

### Authentication not working

Verify user format:
```bash
# htpasswd should produce format like:
# admin:$apr1$H6uskkkW$IgXLP6ewTrSuBkTrqE8wj/
```

Test credentials:
```bash
# Use browser or
curl -u admin:password https://app.example.com/
```

## Log Analysis

### Find errors

```bash
traefikctl logs | grep -i error
traefikctl logs | grep -E "WARN|ERROR"
```

### Real-time monitoring

```bash
traefikctl logs -f
```

### Access log analysis

```bash
# Top 10 requests
traefikctl logs --access | awk '{print $7}' | sort | uniq -c | sort -rn | head

# Slow requests
traefikctl logs --access | awk '$NF > 5 {print}'
```

## Log Rotation Issues

### Logs keep growing — rotation is not running

Check whether automatic rotation is installed:

```bash
traefikctl logs rotation status
```

If neither mode is reported as `INSTALLED`, install one:

```bash
sudo traefikctl logs rotation install                       # systemd timer (default)
sudo traefikctl logs rotation install --mode logrotate      # requires the `logrotate` package
```

### Rotation ran but the file is still huge

The rotation truncates the file to zero, but Traefik may already be
buffering writes. Verify the actual on-disk size after a manual run:

```bash
sudo traefikctl logs rotate --force
ls -lh /var/log/traefik/traefik.log /var/log/traefik/access.log
```

If the size still does not drop, Traefik is likely writing through a
deleted/renamed inode (typical when something other than traefikctl
rotated the file). Restart the service to force it to reopen the path:

```bash
traefikctl service restart
```

### `logrotate not found in PATH` when installing rotation

The `logrotate` mode requires the system package:

```bash
sudo apt install logrotate
```

Or fall back to the self-contained timer mode:

```bash
sudo traefikctl logs rotation install --mode timer
```

### Inspecting the timer

```bash
systemctl list-timers traefikctl-logs-rotate.timer
journalctl -u traefikctl-logs-rotate.service -n 50
sudo systemctl start traefikctl-logs-rotate.service     # run once now
```

## Performance Issues

### High latency

1. Check backend response time:
```bash
curl -w "@curl-format.txt" -o /dev/null -s http://backend:8080
```

2. Check Traefik logs for timeouts:
```bash
traefikctl logs | grep -i timeout
```

3. Increase timeouts in config:
```yaml
http:
  services:
    myapp:
      loadBalancer:
        responseForwarding:
          flushInterval: 100ms
```

### High CPU usage

Enable access log buffering:
```yaml
accessLog:
  bufferingSize: 100
  filters:
    statusCodes:
      - "400-599"
```

## Recovery

### Reset to clean state

```bash
# Stop service
traefikctl service stop

# Backup configs
sudo cp -r /etc/traefik /backup/traefik-$(date +%Y%m%d)

# Generate fresh config
traefikctl config --generate --force

# Restart
traefikctl service start
```

### Emergency stop

```bash
# Stop service
traefikctl service stop

# Or kill manually
sudo pkill traefik
```
