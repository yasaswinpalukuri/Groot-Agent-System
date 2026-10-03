#!/usr/bin/env bash
# Apply every ldap/config/*.ldif to cn=config, in filename order.
# Idempotent as long as each LDIF uses "replace:" (not "add:").
# Runs in its own process, so .env secrets never leak into the caller's shell.
#
# Transport: StartTLS is MANDATORY (-ZZ). There is deliberately NO automatic
# fallback to plaintext: "try TLS, fall back if it fails" is exactly the
# downgrade an attacker exploits (STRIPTLS). The only plaintext path is the
# explicit --bootstrap flag, for a brand-new volume before 20-tls.ldif exists.
set -euo pipefail
cd "$(dirname "$0")"
set -a; . ./.env; set +a

URI="${LDAP_URI:-ldap://127.0.0.1:3389}"   # 127.0.0.1, NOT localhost (libldap TLS hostname quirk)
export LDAPTLS_CACERT="${LDAP_CA:-$(cd .. && pwd)/certs/root_ca.crt}"

TLS_FLAG=(-ZZ)
if [[ "${1:-}" == "--bootstrap" ]]; then
  echo "WARNING: --bootstrap: config admin password crosses loopback in PLAINTEXT" >&2
  TLS_FLAG=()
fi

for f in config/*.ldif; do
  echo "applying $f"
  ldapmodify -x "${TLS_FLAG[@]}" -H "$URI" -D "cn=config,cn=config" \
    -y <(printf %s "$LDAP_CONFIG_ADMIN_PASSWORD") -f "$f"
done
echo "config applied"
