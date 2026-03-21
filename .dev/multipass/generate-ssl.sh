#!/bin/bash
set -e

CERT_DIR="/etc/traefik/certs"
mkdir -p "$CERT_DIR"

echo "Generating self-signed SSL certificate..."

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.crt" \
    -subj "/C=BR/ST=State/L=City/O=Development/CN=*.localhost" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:app1.localhost,DNS:app2.localhost,IP:127.0.0.1"

chown -R traefik:traefik "$CERT_DIR"
chmod 600 "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/server.crt"

echo "Certificate generated at $CERT_DIR"
