#!/usr/bin/env bash
# Apply every ldap/config/*.ldif to cn=config, in filename order.
# Idempotent as long as each LDIF uses "replace:" (not "add:").
# Runs in its own process, so .env secrets never leak into the caller's shell.
set -euo pipefail
cd "$(dirname "$0")"
set -a; . ./.env; set +a

URI="${LDAP_URI:-ldap://127.0.0.1:3389}"
for f in config/*.ldif; do
  echo "applying $f"
  ldapmodify -x -H "$URI" -D "cn=config,cn=config" \
    -y <(printf %s "$LDAP_CONFIG_ADMIN_PASSWORD") -f "$f"
done
echo "config applied"
