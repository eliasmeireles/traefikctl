#!/bin/bash
# Run from the project root. Requires multipass VM 'traefikctl-dev' to be running.
# Usage: bash .dev/multipass/run-e2e-tests.sh

set -euo pipefail

VM_NAME="traefikctl-dev"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VOLUMES_DIR="${SCRIPT_DIR}/.volumes"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== HAProxy Export E2E Tests ==="
echo ""

if ! multipass list 2>/dev/null | grep -q "${VM_NAME}.*Running"; then
    echo "[ERROR] VM '${VM_NAME}' is not running."
    echo "Start it with: cd .dev/multipass && bash setup.sh"
    exit 1
fi

echo "[1/5] Building traefikctl binary..."
cd "$PROJECT_ROOT"
make build
echo "[OK] Build complete"

echo "[2/5] Installing binary in VM..."
multipass transfer "$PROJECT_ROOT/build/traefikctl" "$VM_NAME:/tmp/traefikctl"
multipass exec "$VM_NAME" -- sudo mv /tmp/traefikctl /usr/local/bin/traefikctl
multipass exec "$VM_NAME" -- sudo chmod +x /usr/local/bin/traefikctl
if ! multipass exec "$VM_NAME" -- traefikctl --version > /dev/null 2>&1; then
    echo "[ERROR] Binary is not compatible with VM or failed to run"
    exit 1
fi
echo "[OK] Binary verified"
echo "[OK] Binary installed"

echo "[3/5] Syncing test fixtures..."
mkdir -p "$VOLUMES_DIR/test-fixtures"
cp -r "$SCRIPT_DIR/test-fixtures/"* "$VOLUMES_DIR/test-fixtures/"
cp "$SCRIPT_DIR/test-haproxy-export.sh" "$VOLUMES_DIR/"
echo "[OK] Fixtures synced"

echo "[4/5] Ensuring docker-compose services are up..."
multipass exec "$VM_NAME" -- bash -c "cd /home/ubuntu/traefikctl && docker compose up -d"
sleep 5
echo "[OK] Services started"

echo "[5/5] Running integration tests inside VM..."
echo ""
multipass exec "$VM_NAME" -- sudo bash /home/ubuntu/traefikctl/test-haproxy-export.sh
