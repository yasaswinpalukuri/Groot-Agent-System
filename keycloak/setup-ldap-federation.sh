#!/usr/bin/env bash
# keycloak/setup-ldap-federation.sh — idempotent Keycloak -> OpenLDAP federation (Phase 6.6)
#
# Re-runnable: creates what is missing, updates what exists. CI (Phase 8) runs this.
# Requires an active kcadm session in the container (see docs/learn/phase6.md).
#
# Secret handling: svc-keycloak's password is read from ldap/.env in THIS process
# and sent to kcadm on STDIN (JSON), never on any command line. Tests reference the
# stored credential via Keycloak's mask "**********" + componentId, so the secret is
# substituted server-side and not re-sent.
set -euo pipefail
cd "$(dirname "$0")"
ROOT_DIR="$(cd .. && pwd)"
set -a; . "$ROOT_DIR/ldap/.env"; set +a

REALM=groot
NAME=groot-ldap
BIND_DN="cn=svc-keycloak,ou=services,dc=groot,dc=local"
GOOD_URL="ldaps://openldap:1636"

K() { docker exec -i groot-keycloak /opt/keycloak/bin/kcadm.sh "$@"; }
log() { echo "[setup-ldap] $*"; }
pass=0; fail=0
check() {
  local name=$1 want=$2 got=$3
  if [[ "$got" == "$want" ]]; then printf 'PASS  %-58s (got %s)\n' "$name" "$got"; pass=$((pass+1))
  else printf 'FAIL  %-58s (want %s, got %s)\n' "$name" "$want" "$got"; fail=$((fail+1)); fi
}
verdict() { [[ "$1" == 0 ]] && echo accepted || echo rejected; }

# ---------------------------------------------------------------- 6.6b provider
RID=$(K get "realms/$REALM" --fields id --format csv --noquotes)

provider_json() {
cat <<EOF
{
  "name": "$NAME",
  "providerId": "ldap",
  "providerType": "org.keycloak.storage.UserStorageProvider",
  "parentId": "$RID",
  "config": {
    "enabled": ["true"],
    "priority": ["0"],
    "vendor": ["other"],
    "connectionUrl": ["$GOOD_URL"],
    "startTls": ["false"],
    "useTruststoreSpi": ["always"],
    "connectionPooling": ["true"],
    "connectionTimeout": ["5000"],
    "readTimeout": ["10000"],
    "authType": ["simple"],
    "bindDn": ["$BIND_DN"],
    "bindCredential": ["$LDAP_PW_SVC_KEYCLOAK"],
    "editMode": ["READ_ONLY"],
    "usersDn": ["ou=people,dc=groot,dc=local"],
    "usernameLDAPAttribute": ["uid"],
    "rdnLDAPAttribute": ["uid"],
    "uuidLDAPAttribute": ["entryUUID"],
    "userObjectClasses": ["inetOrgPerson"],
    "searchScope": ["1"],
    "pagination": ["true"],
    "importEnabled": ["true"],
    "syncRegistrations": ["false"],
    "trustEmail": ["false"],
    "batchSizeForSync": ["1000"],
    "fullSyncPeriod": ["-1"],
    "changedSyncPeriod": ["-1"],
    "cachePolicy": ["DEFAULT"],
    "allowKerberosAuthentication": ["false"],
    "useKerberosForPasswordAuthentication": ["false"],
    "validatePasswordPolicy": ["false"]
  }
}
EOF
}

CID=$(K get components -r "$REALM" -q name="$NAME" -q providerId=ldap --fields id --format csv --noquotes || true)
if [[ -z "$CID" ]]; then
  provider_json | K create components -r "$REALM" -f - >/dev/null
  CID=$(K get components -r "$REALM" -q name="$NAME" -q providerId=ldap --fields id --format csv --noquotes)
  log "created LDAP provider $NAME ($CID)"
else
  provider_json | K update "components/$CID" -r "$REALM" -f - >/dev/null
  log "updated LDAP provider $NAME ($CID)"
fi

# ------------------------------------------------------------ 6.6b provider tests
tconn() {   # $1=action $2=url  -> exit code of Keycloak's own LDAP test endpoint
  K create testLDAPConnection -r "$REALM" \
    -s action="$1" -s connectionUrl="$2" -s componentId="$CID" \
    -s bindDn="$BIND_DN" -s bindCredential='**********' \
    -s useTruststoreSpi=always -s startTls=false -s connectionTimeout=5000 >/dev/null 2>&1
  echo $?
}
LDAP_IP=$(docker inspect -f '{{(index .NetworkSettings.Networks "groot-iam").IPAddress}}' groot-openldap)

echo "== provider: connectivity + trust =="
check "connect over LDAPS ($GOOD_URL)"                       0        "$(tconn testConnection "$GOOD_URL")"
check "svc-keycloak authenticates over LDAPS"                0        "$(tconn testAuthentication "$GOOD_URL")"
check "IP not in SANs rejected (ldaps://$LDAP_IP:1636)"      rejected "$(verdict "$(tconn testAuthentication "ldaps://$LDAP_IP:1636")")"
check "plaintext bind rejected (ldap://openldap:1389)"       rejected "$(verdict "$(tconn testAuthentication "ldap://openldap:1389")")"

# ----------------------------------------------------- 6.6c local-user collision
# The Phase 2 local "yasaswin" would collide with LDAP's uid=yasaswin. Rename +
# DISABLE it (reversible), never delete during a migration. Only touch a
# yasaswin WITHOUT a federationLink, so re-runs never rename the LDAP user.
UJSON=$(K get users -r "$REALM" -q username=yasaswin -q exact=true)
if grep -q '"username" : "yasaswin"' <<<"$UJSON" && ! grep -q '"federationLink"' <<<"$UJSON"; then
  LOCAL_ID=$(grep -m1 '"id"' <<<"$UJSON" | cut -d'"' -f4)
  ORIG_EDIT=$(K get "realms/$REALM" --fields editUsernameAllowed --format csv --noquotes || true)
  if [[ "$ORIG_EDIT" != "true" ]]; then
    K update "realms/$REALM" -s editUsernameAllowed=true
    # Restore the realm setting even if the rename fails.
    trap 'K update "realms/$REALM" -s editUsernameAllowed=false; log "restored editUsernameAllowed=false"' EXIT
  fi
  K update "users/$LOCAL_ID" -r "$REALM" -s username=yasaswin-local -s enabled=false
  log "renamed local yasaswin -> yasaswin-local (disabled), id $LOCAL_ID"
  # Close the window immediately; the trap only covers a failed rename.
  if [[ "$ORIG_EDIT" != "true" ]]; then
    K update "realms/$REALM" -s editUsernameAllowed=false
    trap - EXIT
    log "restored editUsernameAllowed=false"
  fi
else
  log "no local (non-federated) yasaswin to rename"
fi

# ------------------------------------------------- 6.6c fix auto-created mapper
# Keycloak's default for non-AD vendors maps firstName -> cn. Our cn is the FULL
# name, so point it at givenName.
FN_ID=$(K get components -r "$REALM" -q parent="$CID" -q name="first name" --fields id --format csv --noquotes)
K update "components/$FN_ID" -r "$REALM" -s 'config."ldap.attribute"=["givenName"]'
log "first name mapper -> givenName"

# -------------------------------------------------------- 6.6c group mapper
group_mapper_json() {
cat <<EOF
{
  "name": "groot-groups",
  "providerId": "group-ldap-mapper",
  "providerType": "org.keycloak.storage.ldap.mappers.LDAPStorageMapper",
  "parentId": "$CID",
  "config": {
    "groups.dn": ["ou=groups,dc=groot,dc=local"],
    "group.name.ldap.attribute": ["cn"],
    "group.object.classes": ["groupOfNames"],
    "preserve.group.inheritance": ["false"],
    "ignore.missing.groups": ["false"],
    "membership.ldap.attribute": ["member"],
    "membership.attribute.type": ["DN"],
    "membership.user.ldap.attribute": ["uid"],
    "mode": ["READ_ONLY"],
    "user.roles.retrieve.strategy": ["LOAD_GROUPS_BY_MEMBER_ATTRIBUTE"],
    "memberof.ldap.attribute": ["memberOf"],
    "drop.non.existing.groups.during.sync": ["false"],
    "groups.path": ["/"]
  }
}
EOF
}
GM_ID=$(K get components -r "$REALM" -q parent="$CID" -q name=groot-groups --fields id --format csv --noquotes || true)
if [[ -z "$GM_ID" ]]; then
  group_mapper_json | K create components -r "$REALM" -f - >/dev/null
  GM_ID=$(K get components -r "$REALM" -q parent="$CID" -q name=groot-groups --fields id --format csv --noquotes)
  log "created group mapper groot-groups ($GM_ID)"
else
  group_mapper_json | K update "components/$GM_ID" -r "$REALM" -f - >/dev/null
  log "updated group mapper groot-groups ($GM_ID)"
fi

# --------------------------------------------------------------- 6.6c sync
# Users first: group memberships reference users.
log "user sync:  $(K create "user-storage/$CID/sync?action=triggerFullSync" -r "$REALM" -o | tr -d '\n ')"
log "group sync: $(K create "user-storage/$CID/mappers/$GM_ID/sync?direction=fedToKeycloak" -r "$REALM" -o | tr -d '\n ')"

# ------------------------------------------------------- 6.6c roles -> groups
# Directory GROUPS (how people are organised) map to IdP ROLES (what the app
# allows). Roles are deliberately NOT composite: admins are in both LDAP groups,
# and orthogonal roles mean revoking admin never revokes basic access.
ensure_role() {
  K get "roles/$1" -r "$REALM" >/dev/null 2>&1 || { K create roles -r "$REALM" -s name="$1" -s description="$2"; log "created role $1"; }
}
ensure_role agent_access "May call Groot agent APIs through the gateway"
ensure_role groot_admin  "Administrative access to Groot"
K add-roles -r "$REALM" --gpath /groot-users  --rolename agent_access
K add-roles -r "$REALM" --gpath /groot-admins --rolename groot_admin
log "granted agent_access -> /groot-users, groot_admin -> /groot-admins"

# ------------------------------------------------------------- 6.6c tests
uid_of()  { K get users -r "$REALM" -q username="$1" -q exact=true --fields id --format csv --noquotes; }
field()   { K get "users/$1" -r "$REALM" --fields "$2" --format csv --noquotes; }
eff()     { K get "users/$1/role-mappings/realm/composite" -r "$REALM" --fields name --format csv --noquotes; }
has()     { eff "$1" | grep -qx "$2" && echo yes || echo no; }
YID=$(uid_of yasaswin); TID=$(uid_of tester)

echo "== federation: users, attributes, collision =="
check "yasaswin is federated from groot-ldap"          "$CID"  "$(field "$YID" federationLink)"
check "tester is federated from groot-ldap"            "$CID"  "$(field "$TID" federationLink)"
check "local account is yasaswin-local and disabled"   "false" "$(K get users -r "$REALM" -q username=yasaswin-local -q exact=true --fields enabled --format csv --noquotes)"
check "yasaswin firstName comes from givenName"        "Yasaswin" "$(field "$YID" firstName)"

echo "== RBAC: LDAP group -> Keycloak role =="
check "yasaswin has agent_access"   yes "$(has "$YID" agent_access)"
check "yasaswin has groot_admin"    yes "$(has "$YID" groot_admin)"
check "tester has agent_access"     yes "$(has "$TID" agent_access)"
check "tester LACKS groot_admin"    no  "$(has "$TID" groot_admin)"

# ------------------------------------------ 6.6d audience scope (shared, once)
# Keycloak does NOT put the client itself into "aud" by default; the gateway's
# validator requires aud to contain groot-gateway. One client scope defines
# "tokens meant for the gateway"; attach it to every client that calls it.
SCOPE=groot-gateway-audience
SID=$(K get client-scopes -r "$REALM" --fields id,name --format csv --noquotes | awk -F, -v n="$SCOPE" '$2==n{print $1}')
if [[ -z "$SID" ]]; then
  K create client-scopes -r "$REALM" -s name="$SCOPE" -s protocol=openid-connect \
    -s 'attributes."include.in.token.scope"=false' -s 'attributes."display.on.consent.screen"=false' >/dev/null
  SID=$(K get client-scopes -r "$REALM" --fields id,name --format csv --noquotes | awk -F, -v n="$SCOPE" '$2==n{print $1}')
  log "created client scope $SCOPE ($SID)"
fi
if ! K get "client-scopes/$SID/protocol-mappers/models" -r "$REALM" --fields name --format csv --noquotes | grep -qx groot-gateway-aud; then
  K create "client-scopes/$SID/protocol-mappers/models" -r "$REALM" \
    -s name=groot-gateway-aud -s protocol=openid-connect -s protocolMapper=oidc-audience-mapper \
    -s 'config."included.client.audience"=groot-gateway' \
    -s 'config."access.token.claim"=true' -s 'config."id.token.claim"=false' >/dev/null
  log "created audience mapper groot-gateway-aud (access token only)"
fi

# ------------------------------------------------------ 6.6d groot-ci client
# Password grant (ROPC) is deprecated in OAuth 2.1 because it hands user
# passwords to apps. It exists ONLY here: a confidential, CI-only client with
# every other flow disabled. Humans log in via PKCE through groot-gateway.
CI_ID=$(K get clients -r "$REALM" -q clientId=groot-ci --fields id --format csv --noquotes || true)
if [[ -z "$CI_ID" ]]; then
  K create clients -r "$REALM" -s clientId=groot-ci -s enabled=true -s publicClient=false \
    -s standardFlowEnabled=false -s implicitFlowEnabled=false -s directAccessGrantsEnabled=true \
    -s serviceAccountsEnabled=false \
    -s 'description=CI-only password-grant client (dev realm). Humans use PKCE via groot-gateway.' >/dev/null
  CI_ID=$(K get clients -r "$REALM" -q clientId=groot-ci --fields id --format csv --noquotes)
  log "created client groot-ci ($CI_ID)"
fi
GW_ID=$(K get clients -r "$REALM" -q clientId=groot-gateway --fields id --format csv --noquotes)
for cid in "$GW_ID" "$CI_ID"; do K update "clients/$cid/default-client-scopes/$SID" -r "$REALM"; done
log "attached $SCOPE as default scope on groot-gateway and groot-ci"

# ---------------------------------------------------------- 6.6d token tests
# Secrets reach curl via --data-urlencode name@<(printf ...): argv shows
# /dev/fd/N, never the value; URL-encoding keeps &,+,= in passwords intact.
TOKEN_URL="http://127.0.0.1:8090/realms/$REALM/protocol/openid-connect/token"
CI_SECRET=$(K get "clients/$CI_ID/client-secret" -r "$REALM" --fields value --format csv --noquotes)
token_resp() {   # $1=username $2=password
  curl -s "$TOKEN_URL" -d grant_type=password -d client_id=groot-ci -d scope=openid \
    --data-urlencode client_secret@<(printf %s "$CI_SECRET") \
    --data-urlencode username="$1" --data-urlencode password@<(printf %s "$2")
}
claim() {        # stdin: token-endpoint JSON; $1: dotted claim path. Lists -> space-joined.
  python3 -c '
import sys, json, base64
r = json.load(sys.stdin)
if "access_token" not in r:
    print("ERROR:" + r.get("error", "?")); sys.exit()
p = r["access_token"].split(".")[1]; p += "=" * (-len(p) % 4)
v = json.loads(base64.urlsafe_b64decode(p))
for k in sys.argv[1].split("."):
    v = v.get(k) if isinstance(v, dict) else None
print(" ".join(v) if isinstance(v, list) else v)
' "$1"
}
inlist() { tr ' ' '\n' | grep -qx "$1" && echo yes || echo no; }

RESP=$(token_resp tester "$LDAP_PW_TESTER")
echo "== tokens: what the gateway will actually receive (tester via groot-ci) =="
check "token issued for tester (LDAP password verified)"  issued "$(claim sub <<<"$RESP" | grep -q '^ERROR' && echo failed || echo issued)"
check "azp is groot-ci"                                     groot-ci "$(claim azp <<<"$RESP")"
check "aud contains groot-gateway (audience mapper)"        yes "$(claim aud <<<"$RESP" | inlist groot-gateway)"
check "preferred_username is tester"                        tester "$(claim preferred_username <<<"$RESP")"
check "token roles include agent_access"                    yes "$(claim realm_access.roles <<<"$RESP" | inlist agent_access)"
check "token roles EXCLUDE groot_admin"                     no  "$(claim realm_access.roles <<<"$RESP" | inlist groot_admin)"
check "wrong password refused"                              ERROR:invalid_grant "$(token_resp tester definitely-wrong | claim sub)"
check "disabled yasaswin-local refused"                     ERROR:invalid_grant "$(token_resp yasaswin-local anything | claim sub)"
log "token iss = $(claim iss <<<"$RESP")  (host-dependent until KC_HOSTNAME is pinned in 6.7)"

echo
echo "$pass passed, $fail failed"
exit $(( fail > 0 ))
