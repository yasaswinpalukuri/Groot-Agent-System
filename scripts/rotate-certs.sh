#!/usr/bin/env bash
# scripts/rotate-certs.sh — Zero-downtime certificate rotation for Groot IAM
#
# WHY zero-downtime rotation?
# Stopping the gateway to swap certs would break all active connections.
# The rotation strategy: issue the new cert BEFORE the old one expires,
# write it to a staging location, then atomically replace the live cert
# and send SIGHUP to the gateway process (which does a graceful TLS reload).
#
# HOW Go's crypto/tls does graceful reload:
# Our gateway server holds a *tls.Config with a GetCertificate callback.
# On SIGHUP, the gateway re-reads the cert files and updates the callback.
# Existing connections use the old cert; new connections get the new cert.
# No connections are dropped.
#
# Usage:
#   ./scripts/rotate-certs.sh gateway     # rotate gateway cert
#   ./scripts/rotate-certs.sh agent       # rotate agent_service cert
#
# Prerequisites:
#   - step CLI installed (or run inside groot-step-ca container)
#   - CA_URL, CA_ROOT, and STEP_PROVISIONER set (or defaults used)
#   - Gateway process running with PID in /run/groot-gateway.pid

set -euo pipefail

CERT_NAME="${1:-gateway}"
CERT_DIR="${CERT_DIR:-/home/groot/code/groot-iam/certs}"
CA_URL="${CA_URL:-https://localhost:9000}"
CA_ROOT="${CA_ROOT:-${CERT_DIR}/root_ca.crt}"
PROVISIONER="${STEP_PROVISIONER:-groot-admin}"
CERT_DURATION="${CERT_DURATION:-24h}"
GATEWAY_PID_FILE="/run/groot-gateway.pid"

CERT_FILE="${CERT_DIR}/${CERT_NAME}.crt"
KEY_FILE="${CERT_DIR}/${CERT_NAME}.key"
STAGING_CERT="${CERT_DIR}/${CERT_NAME}.crt.new"
STAGING_KEY="${CERT_DIR}/${CERT_NAME}.key.new"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] rotate-certs: $*"; }

log "Starting rotation for ${CERT_NAME}"

# Step 1: Issue new cert to staging files
# WHY staging? If cert issuance fails, the live cert is untouched.
log "Issuing new cert from ${CA_URL}"
step ca certificate "${CERT_NAME}" "${STAGING_CERT}" "${STAGING_KEY}" \
    --ca-url "${CA_URL}" \
    --root "${CA_ROOT}" \
    --provisioner "${PROVISIONER}" \
    --not-after "${CERT_DURATION}" \
    --force

# Step 2: Verify the new cert before replacing the live one
log "Verifying new cert"
step certificate inspect "${STAGING_CERT}" --short
NOT_AFTER=$(step certificate inspect "${STAGING_CERT}" --format json | python3 -c "import json,sys; print(json.load(sys.stdin)['validity']['notAfter'])")
log "New cert valid until ${NOT_AFTER}"

# Step 3: Atomic replace — mv is atomic on the same filesystem
log "Replacing live cert (atomic)"
mv "${STAGING_CERT}" "${CERT_FILE}"
mv "${STAGING_KEY}" "${KEY_FILE}"

# Step 4: Signal gateway to reload TLS config
if [[ -f "${GATEWAY_PID_FILE}" ]]; then
    GW_PID=$(cat "${GATEWAY_PID_FILE}")
    if kill -0 "${GW_PID}" 2>/dev/null; then
        log "Sending SIGHUP to gateway (pid ${GW_PID})"
        kill -HUP "${GW_PID}"
        sleep 2
        log "Gateway reloaded"
    else
        log "WARNING: gateway pid ${GW_PID} not running — cert replaced but no reload"
    fi
else
    log "WARNING: no gateway pid file at ${GATEWAY_PID_FILE} — cert replaced but no reload"
fi

log "Rotation complete for ${CERT_NAME}"
