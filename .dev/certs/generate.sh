#!/bin/sh
# Generates a throwaway CA and a Mailpit server certificate (SAN: mailpit, localhost) so the
# mailer's implicit-TLS SMTP client can verify Mailpit like a real server. Idempotent.
set -eu

CA_DIR=/certs/ca
SERVER_DIR=/certs/server

if [ -f "$SERVER_DIR/mailpit.pem" ]; then
  echo "certificates already present"
  exit 0
fi

apk add --no-cache openssl >/dev/null
mkdir -p "$CA_DIR" "$SERVER_DIR" /tmp/ca-key

openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
  -subj "/CN=gomailer-dev test CA" \
  -keyout /tmp/ca-key/ca-key.pem -out "$CA_DIR/ca.pem" 2>/dev/null

openssl req -newkey rsa:2048 -nodes \
  -subj "/CN=mailpit" \
  -keyout "$SERVER_DIR/mailpit-key.pem" -out /tmp/mailpit.csr 2>/dev/null

printf "subjectAltName=DNS:mailpit,DNS:localhost\nextendedKeyUsage=serverAuth\n" > /tmp/ext.cnf
openssl x509 -req -in /tmp/mailpit.csr -days 3650 \
  -CA "$CA_DIR/ca.pem" -CAkey /tmp/ca-key/ca-key.pem -CAcreateserial \
  -extfile /tmp/ext.cnf -out "$SERVER_DIR/mailpit.pem" 2>/dev/null

chmod 644 "$SERVER_DIR"/*.pem "$CA_DIR/ca.pem"
echo "certificates generated"
