#!/bin/bash

set -e

VM_NAME="traefikctl-dev"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VOLUMES_DIR="${SCRIPT_DIR}/.volumes"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Traefik Control (traefikctl) Development Environment ==="
echo ""

check_multipass() {
    if ! command -v multipass &> /dev/null; then
        echo "Multipass is not installed"
        echo "Install it from: https://multipass.run/"
        exit 1
    fi
    echo "[OK] Multipass is installed"
}

get_ssh_key() {
    if [ -f "$HOME/.ssh/id_rsa.pub" ]; then
        export SSH_PUBLIC_KEY=$(cat "$HOME/.ssh/id_rsa.pub")
    elif [ -f "$HOME/.ssh/id_ed25519.pub" ]; then
        export SSH_PUBLIC_KEY=$(cat "$HOME/.ssh/id_ed25519.pub")
    else
        echo "No SSH public key found"
        echo "Generate one with: ssh-keygen -t ed25519"
        exit 1
    fi
    echo "[OK] SSH key found"
}

create_vm() {
    echo ""
    echo "Creating VM: $VM_NAME"

    OUT_DIR="$SCRIPT_DIR/.out"
    mkdir -p "$OUT_DIR"
    CLOUD_INIT_FILE="$OUT_DIR/cloud-init-generated.yaml"

    sed "s|\${SSH_PUBLIC_KEY}|${SSH_PUBLIC_KEY}|g" "$SCRIPT_DIR/cloud-init.yaml" > "$CLOUD_INIT_FILE"

    ARCH=$(uname -m)
    UBUNTU_IMAGE="22.04"

    if [[ "$OSTYPE" == "darwin"* ]] && [[ "$ARCH" == "arm64" ]]; then
        echo "Detected macOS ARM64 - using ARM-compatible Ubuntu image"
    fi

    multipass launch \
        --name "$VM_NAME" \
        --cpus 2 \
        --memory 2G \
        --disk 10G \
        --mount "$VOLUMES_DIR:/home/ubuntu/traefikctl" \
        --cloud-init "$CLOUD_INIT_FILE" \
        "$UBUNTU_IMAGE"

    echo "[OK] VM created"
}

wait_for_vm() {
    echo ""
    echo "Waiting for VM to be ready..."
    sleep 10

    for i in {1..30}; do
        if multipass exec "$VM_NAME" -- systemctl is-active docker &> /dev/null; then
            echo "[OK] VM is ready"
            return 0
        fi
        echo "Waiting... ($i/30)"
        sleep 5
    done

    echo "VM did not become ready in time"
    exit 1
}

prepare_volume() {
    echo ""
    echo "Preparing volume directory..."

    mkdir -p "$VOLUMES_DIR/dynamic"
    mkdir -p "$VOLUMES_DIR/html/app1"
    mkdir -p "$VOLUMES_DIR/html/app2"

    # Build traefikctl
    cd "$PROJECT_ROOT"
    make build

    # Copy files to volume
    cp "$SCRIPT_DIR/dynamic-config.yaml" "$VOLUMES_DIR/dynamic/services.yaml"
    cp "$SCRIPT_DIR/docker-compose.yml" "$VOLUMES_DIR/"
    cp -r "$SCRIPT_DIR/html" "$VOLUMES_DIR/"
    cp "$SCRIPT_DIR/generate-ssl.sh" "$VOLUMES_DIR/"
    mkdir -p "$VOLUMES_DIR/test-fixtures"
    cp -r "$SCRIPT_DIR/test-fixtures/"* "$VOLUMES_DIR/test-fixtures/"
    cp "$SCRIPT_DIR/test-haproxy-export.sh" "$VOLUMES_DIR/" 2>/dev/null || true

    echo "[OK] Volume prepared at: $VOLUMES_DIR"
}

install_traefikctl() {
    echo ""
    echo "Installing traefikctl binary..."

    multipass transfer "$PROJECT_ROOT/build/traefikctl" "$VM_NAME:/tmp/traefikctl"
    multipass exec "$VM_NAME" -- sudo mv /tmp/traefikctl /usr/local/bin/traefikctl
    multipass exec "$VM_NAME" -- sudo chmod +x /usr/local/bin/traefikctl

    echo "[OK] traefikctl installed to /usr/local/bin"
}

install_traefik() {
    echo ""
    echo "Installing Traefik via traefikctl..."

    multipass exec "$VM_NAME" -- sudo traefikctl install

    echo "[OK] Traefik installed"
}

setup_traefik_config() {
    echo ""
    echo "Setting up Traefik configuration..."

    # Generate default static config
    multipass exec "$VM_NAME" -- sudo traefikctl config --generate

    # Link dynamic config directory to mounted volume
    multipass exec "$VM_NAME" -- sudo rm -rf /etc/traefik/dynamic
    multipass exec "$VM_NAME" -- sudo ln -sf /home/ubuntu/traefikctl/dynamic /etc/traefik/dynamic

    # Allow non-ubuntu users (e.g. traefik service user) to traverse /home/ubuntu
    multipass exec "$VM_NAME" -- sudo chmod o+x /home/ubuntu

    echo "[OK] Configuration linked from volume"
}

setup_ssl() {
    echo ""
    echo "Setting up SSL certificate..."

    multipass exec "$VM_NAME" -- chmod +x /home/ubuntu/traefikctl/generate-ssl.sh
    multipass exec "$VM_NAME" -- sudo bash /home/ubuntu/traefikctl/generate-ssl.sh

    echo "[OK] SSL certificate generated"
}

install_service() {
    echo ""
    echo "Installing systemd service..."

    multipass exec "$VM_NAME" -- sudo traefikctl service install

    echo "[OK] Systemd service installed"
}

start_docker_containers() {
    echo ""
    echo "Starting Docker containers..."

    multipass exec "$VM_NAME" -- bash -c "cd /home/ubuntu/traefikctl && docker compose up -d"
    sleep 5
    multipass exec "$VM_NAME" -- docker ps

    echo "[OK] Docker containers started"
}

start_traefik() {
    echo ""
    echo "Starting Traefik service..."

    multipass exec "$VM_NAME" -- sudo systemctl start traefikctl
    sleep 2

    if multipass exec "$VM_NAME" -- systemctl is-active traefikctl &> /dev/null; then
        echo "[OK] Traefik is running"
    else
        echo "[WARN] Traefik may not have started correctly"
        multipass exec "$VM_NAME" -- sudo journalctl -u traefikctl --no-pager -n 20
    fi
}

show_info() {
    VM_IP=$(multipass info "$VM_NAME" | grep IPv4 | awk '{print $2}')

    echo ""
    echo "=========================================="
    echo "Development environment is ready!"
    echo "=========================================="
    echo ""
    echo "VM Name: $VM_NAME"
    echo "VM IP: $VM_IP"
    echo ""
    echo "Shared volume:"
    echo "  Host: $VOLUMES_DIR"
    echo "  VM:   /home/ubuntu/traefikctl"
    echo "  Dynamic configs: /etc/traefik/dynamic -> /home/ubuntu/traefikctl/dynamic"
    echo ""
    echo "Test applications:"
    echo "  App 1: http://$VM_IP:8081 (direct)"
    echo "  App 2: http://$VM_IP:8082 (direct)"
    echo ""
    echo "Traefik proxy:"
    echo "  http://$VM_IP (default -> app1)"
    echo "  curl -H 'Host: app1.localhost' http://$VM_IP  (-> app1)"
    echo "  curl -H 'Host: app2.localhost' http://$VM_IP  (-> app2)"
    echo ""
    echo "Useful commands:"
    echo "  multipass shell $VM_NAME"
    echo "  multipass exec $VM_NAME -- traefikctl check"
    echo "  multipass exec $VM_NAME -- traefikctl config --view"
    echo "  multipass exec $VM_NAME -- sudo traefikctl resource add --domain test.localhost --address 127.0.0.1:8082 --name test"
    echo ""
    echo "Add a new domain route:"
    echo "  traefikctl resource add --domain myapp.localhost --address 127.0.0.1:8081 --name myapp"
    echo "  (Traefik auto-reloads - no restart needed!)"
    echo ""
    echo "Edit configs on host in $VOLUMES_DIR/dynamic/ and they sync to VM automatically!"
    echo "=========================================="
}

main() {
    check_multipass
    get_ssh_key

    VM_EXISTS=false
    if multipass list | grep -q "$VM_NAME"; then
        VM_EXISTS=true
        echo ""
        echo "VM '$VM_NAME' already exists"
        echo ""
        echo "Options:"
        echo "  1) Configure existing VM (update binaries and configs)"
        echo "  2) Delete and recreate VM"
        echo "  3) Abort"
        echo ""
        read -p "Choose option (1/2/3): " -n 1 -r
        echo

        case $REPLY in
            1)
                echo "Configuring existing VM..."
                ;;
            2)
                echo "Deleting and recreating VM..."
                multipass delete "$VM_NAME"
                multipass purge
                VM_EXISTS=false
                ;;
            *)
                echo "Aborted"
                exit 0
                ;;
        esac
    fi

    prepare_volume

    if [ "$VM_EXISTS" = false ]; then
        create_vm
        wait_for_vm
    fi

    install_traefikctl

    if [ "$VM_EXISTS" = false ]; then
        install_traefik
        setup_traefik_config
        start_docker_containers
        setup_ssl
        install_service
        start_traefik
    else
        echo ""
        echo "Restarting Docker containers..."
        multipass exec "$VM_NAME" -- bash -c "cd /home/ubuntu/traefikctl && docker compose restart"

        echo ""
        echo "Restarting Traefik..."
        multipass exec "$VM_NAME" -- sudo systemctl restart traefikctl
    fi

    show_info
}

main "$@"
