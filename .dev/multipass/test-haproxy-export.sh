#!/bin/bash
# Runs INSIDE the multipass VM as root.
# Prerequisites: traefikctl installed, Traefik running, docker-compose apps up.

set -euo pipefail

PASS=0
FAIL=0
FIXTURE="/home/ubuntu/traefikctl/test-fixtures/haproxy-test.cfg"
OUT_DIR="/tmp/haproxy-export-test"
OUT_DIR_B64="/tmp/haproxy-export-test-b64"
DYN_DIR="/etc/traefik/dynamic"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }

assert_file_exists() {
    local file="$1" label="$2"
    if [ -f "$file" ]; then pass "$label"; else fail "$label — file not found: $file"; fi
}

assert_file_missing() {
    local file="$1" label="$2"
    if [ ! -f "$file" ]; then pass "$label"; else fail "$label — file should not exist: $file"; fi
}

assert_contains() {
    local file="$1" pattern="$2" label="$3"
    if grep -q "$pattern" "$file" 2>/dev/null; then pass "$label"; else fail "$label — pattern '$pattern' not found in $file"; fi
}

assert_http_status() {
    local url="$1" header="$2" expected_status="$3" label="$4"
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" -H "$header" "$url" 2>/dev/null || echo "000")
    if [ "$status" = "$expected_status" ]; then
        pass "$label (HTTP $status)"
    else
        fail "$label — expected HTTP $expected_status, got $status"
    fi
}

echo ""
echo "========================================"
echo " HAProxy Export Integration Tests"
echo "========================================"
echo ""

# ── Section 1: Export from file ──────────────────────────────────────────────
echo "--- Section 1: Export from file ---"

rm -rf "$OUT_DIR"
EXPORT_OUTPUT=$(traefikctl haproxy export --file "$FIXTURE" --output-dir "$OUT_DIR" 2>&1)

assert_file_exists "$OUT_DIR/hapctl-test-http.yaml" "HTTP frontend YAML is created"
assert_file_missing "$OUT_DIR/hapctl-test-http-dup.yaml" "Duplicate-port frontend is NOT created"
assert_file_exists "$OUT_DIR/hapctl-test-tcp.yaml" "TCP listen YAML is created"

# ── Section 2: YAML content validation ───────────────────────────────────────
echo ""
echo "--- Section 2: YAML content validation ---"

assert_contains "$OUT_DIR/hapctl-test-http.yaml" "app1.localhost" "HTTP YAML has app1 Host rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "app2.localhost" "HTTP YAML has app2 Host rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "PathPrefix" "HTTP YAML has default PathPrefix rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "127.0.0.1:8081" "HTTP YAML has app1 backend address"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "127.0.0.1:8082" "HTTP YAML has app2 backend address"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "web" "HTTP YAML references 'web' entrypoint"

assert_contains "$OUT_DIR/hapctl-test-tcp.yaml" "HostSNI" "TCP YAML has HostSNI rule"
assert_contains "$OUT_DIR/hapctl-test-tcp.yaml" "127.0.0.1:7001" "TCP YAML has correct backend address"

# ── Section 3: Port conflict warning ─────────────────────────────────────────
echo ""
echo "--- Section 3: Port conflict warning ---"

if echo "$EXPORT_OUTPUT" | grep -qi "port 80.*skip\|skip.*port 80\|already used"; then
    pass "Warning printed for duplicate port 80"
else
    fail "No warning found for duplicate port 80 — output was: $EXPORT_OUTPUT"
fi

# ── Section 4: Base64 input produces identical output ────────────────────────
echo ""
echo "--- Section 4: Base64 input ---"

rm -rf "$OUT_DIR_B64"
B64=$(base64 -w0 "$FIXTURE")
traefikctl haproxy export --base64 "$B64" --output-dir "$OUT_DIR_B64" > /dev/null 2>&1

assert_file_exists "$OUT_DIR_B64/hapctl-test-http.yaml" "base64 input creates HTTP frontend YAML"
assert_file_missing "$OUT_DIR_B64/hapctl-test-http-dup.yaml" "base64 input also skips duplicate-port frontend"
assert_file_exists "$OUT_DIR_B64/hapctl-test-tcp.yaml" "base64 input creates TCP YAML"

if diff -q "$OUT_DIR/hapctl-test-http.yaml" "$OUT_DIR_B64/hapctl-test-http.yaml" > /dev/null 2>&1; then
    pass "base64 output matches file input output"
else
    fail "base64 output differs from file input output"
fi

# ── Section 5: Deploy to Traefik and validate HTTP proxy ─────────────────────
echo ""
echo "--- Section 5: Live Traefik proxy routing ---"

BACKUP_DIR="/tmp/traefik-dynamic-backup-$(date +%s)"
cp -r "$DYN_DIR" "$BACKUP_DIR" 2>/dev/null || true

# Replace dynamic dir contents with exported config
rm -f "$DYN_DIR"/*.yaml 2>/dev/null || true
cp "$OUT_DIR/hapctl-test-http.yaml" "$DYN_DIR/"

# Wait for Traefik file watcher to reload (Traefik watches for changes)
sleep 3

if systemctl is-active traefikctl > /dev/null 2>&1; then
    pass "Traefik service is still active after config deploy"
else
    fail "Traefik service is not active after config deploy"
fi

assert_http_status "http://127.0.0.1" "Host: app1.localhost" "200" "app1.localhost routes to app1"
assert_http_status "http://127.0.0.1" "Host: app2.localhost" "200" "app2.localhost routes to app2"
assert_http_status "http://127.0.0.1" "Host: unknown.localhost" "200" "default backend serves catch-all"

APP1_BODY=$(curl -s -H "Host: app1.localhost" "http://127.0.0.1" 2>/dev/null || echo "CURL_FAILED")
APP2_BODY=$(curl -s -H "Host: app2.localhost" "http://127.0.0.1" 2>/dev/null || echo "CURL_FAILED")

if [ "$APP1_BODY" != "$APP2_BODY" ] && [ "$APP1_BODY" != "CURL_FAILED" ] && [ "$APP2_BODY" != "CURL_FAILED" ]; then
    pass "app1 and app2 return different responses (routing is working)"
else
    fail "app1 and app2 responses identical or curl failed — routing may not be working"
fi

# ── Restore original dynamic config ──────────────────────────────────────────
rm -f "$DYN_DIR"/*.yaml 2>/dev/null || true
cp "$BACKUP_DIR"/*.yaml "$DYN_DIR/" 2>/dev/null || true
sleep 1

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "========================================"
echo " Results: ${PASS} passed, ${FAIL} failed"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
