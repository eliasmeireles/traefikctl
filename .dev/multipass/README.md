# traefikctl Development Environment

Multipass-based development VM with Traefik, Docker, and test applications.

## Quick Start

```bash
cd .dev/multipass
./setup.sh
```

This creates a VM with:
- Traefik installed and running as systemd service
- traefikctl CLI installed globally
- 2 nginx test containers (ports 8081, 8082)
- Domain-based routing configured (app1.localhost, app2.localhost)
- Shared volume for live config editing

## Testing

```bash
# Get VM IP
VM_IP=$(multipass info traefikctl-dev | grep IPv4 | awk '{print $2}')

# Test domain routing
curl -H "Host: app1.localhost" http://$VM_IP
curl -H "Host: app2.localhost" http://$VM_IP

# Default route (no Host header)
curl http://$VM_IP

# Add a new route (auto-reloads!)
multipass exec traefikctl-dev -- sudo traefikctl resource add \
  --domain test.localhost --address 127.0.0.1:8082 --name test
```

## File Sync

Edit files in `.volumes/dynamic/` on the host and they sync to the VM automatically.
Traefik watches the dynamic config directory and applies changes without restart.

## VM Management

```bash
multipass shell traefikctl-dev      # SSH into VM
multipass stop traefikctl-dev       # Stop VM
multipass start traefikctl-dev      # Start VM
multipass delete traefikctl-dev     # Delete VM
multipass purge                     # Remove deleted VMs
```
