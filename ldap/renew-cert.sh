#!/usr/bin/env bash
# ldap/renew-cert.sh — renew slapd's TLS cert via step-ca and hot-reload slapd.
#
# Run hourly by groot-ldap-renew.timer. Safe to run at any time:
#  - step renews ONLY if the cert expires within RENEW_WINDOW (default 8h =
#    last third of a 24h cert), so ~8 hourly attempts before expiry.
#  - Reload is STATE-driven: it runs whenever the serial slapd SERVES differs
#    from the serial ON DISK. Self-healing if a previous run renewed but died
#    before reloading.
#  - Reload is VERIFIED: re-check the served serial; exit 1 if unchanged.
#
# Why re-apply cn=config to reload: proven in step 6.5e — slapd does not watch
# its cert files, but any modify of olcTLS* rebuilds the TLS context live.
#
# Auth to the CA is mTLS with the CURRENT cert+key: no CA secret on this host.
# An already-expired cert is refused by the CA by design; recovery is a manual
# re-issue (CSR flow, step 6.5b).
set -euo pipefail
cd "$(dirname "$0")"

STEP_IMAGE="${STEP_IMAGE:-smallstep/step-cli:0.26.0}"
CA_URL="${CA_URL:-https://localhost:9000}"
ROOT="$(cd .. && pwd)/certs/root_ca.crt"
RENEW_WINDOW="${RENEW_WINDOW:-8h}"

log() { echo "[$(date -u +%FT%TZ)] ldap-renew: $*"; }
# Read the cert as slapd's own uid (tls/ is 700, owned by 1001): no sudo needed.
file_cert()     { docker exec groot-openldap cat /tls/ldap.crt; }
file_serial()   { file_cert | openssl x509 -noout -serial | cut -d= -f2; }
file_enddate()  { file_cert | openssl x509 -noout -enddate | cut -d= -f2; }
served_serial() {
  echo | timeout 5 openssl s_client -connect 127.0.0.1:6636 -CAfile "$ROOT" -verify_ip 127.0.0.1 2>/dev/null \
    | openssl x509 -noout -serial | cut -d= -f2
}

before=$(file_serial)
docker run --rm --user 1001:1001 --network host -e STEPPATH=/tmp/step \
  -v "$PWD/tls:/tls" -v "$ROOT:/ca/root_ca.crt:ro" \
  "$STEP_IMAGE" \
  step ca renew /tls/ldap.crt /tls/ldap.key --ca-url "$CA_URL" --root /ca/root_ca.crt \
    --expires-in "$RENEW_WINDOW" --force
after=$(file_serial)

if [[ "$before" != "$after" ]]; then log "renewed: $before -> $after"
else log "not due (renews in final $RENEW_WINDOW)"; fi

served=$(served_serial || true)
if [[ "$served" != "$after" ]]; then
  log "slapd serving '${served:-<none>}', disk has $after: reloading via cn=config"
  ./apply-config.sh >/dev/null
  served=$(served_serial || true)
  if [[ "$served" != "$after" ]]; then
    log "ERROR: reload failed, slapd still serving '${served:-<none>}'"
    exit 1
  fi
  log "reloaded: slapd now serving $served"
fi
log "ok: serving $served, expires $(file_enddate)"
