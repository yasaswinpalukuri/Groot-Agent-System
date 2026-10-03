#!/usr/bin/env bash
# Groot IAM — OpenLDAP security regression test (ACLs + transport).
# Every check states the EXPECTED secure outcome. Exit 1 on any failure, so
# CI (Phase 8) can gate on it.
#
# All normal checks run over StartTLS (-ZZ) with our root CA. The transport
# section deliberately also tries plaintext, a wrong trust store, and a name
# outside the SANs, each of which MUST be rejected.
set -uo pipefail
cd "$(dirname "$0")/.."
set -a; . ./.env; set +a

L="${LDAP_URI:-ldap://127.0.0.1:3389}"      # 127.0.0.1, NOT localhost (libldap TLS hostname quirk)
LS="${LDAPS_URI:-ldaps://127.0.0.1:6636}"
export LDAPTLS_CACERT="${LDAP_CA:-$(cd .. && pwd)/certs/root_ca.crt}"
B="dc=groot,dc=local"
T="uid=tester,ou=people,$B";        Y="uid=yasaswin,ou=people,$B"
K="cn=svc-keycloak,ou=services,$B"; G="cn=svc-gateway,ou=services,$B"
pass=0; fail=0

as()    { local dn=$1 pw=$2 cmd=$3; shift 3; "$cmd" -x -ZZ -H "$L"  -D "$dn" -y <(printf %s "$pw") "$@"; }
plain() { local dn=$1 pw=$2 cmd=$3; shift 3; "$cmd" -x     -H "$L"  -D "$dn" -y <(printf %s "$pw") "$@"; }
ldaps() { local dn=$1 pw=$2 cmd=$3; shift 3; "$cmd" -x     -H "$LS" -D "$dn" -y <(printf %s "$pw") "$@"; }
srch()  { local dn=$1 pw=$2; shift 2; as "$dn" "$pw" ldapsearch -LLL -o ldif-wrap=no "$@" 2>/dev/null; }
count() { grep -c "$1" || true; }
rc()    { "$@" >/dev/null 2>&1; echo $?; }
verdict() { [[ "$1" == 0 ]] && echo accepted || echo rejected; }
check() {
  local name=$1 want=$2 got=$3
  if [[ "$got" == "$want" ]]; then printf 'PASS  %-55s (got %s)\n' "$name" "$got"; ((pass++))
  else printf 'FAIL  %-55s (want %s, got %s)\n' "$name" "$want" "$got"; ((fail++)); fi
}

echo "== transport: encryption is mandatory and verified =="
check "plaintext bind refused (13 confidentialityRequired)" 13 "$(rc plain "$T" "$LDAP_PW_TESTER" ldapwhoami)"
check "LDAPS bind on 6636 works"           0  "$(rc ldaps "$T" "$LDAP_PW_TESTER" ldapwhoami)"
check "untrusted CA rejected (chain check)" rejected "$(verdict "$(rc env LDAPTLS_CACERT=/etc/ssl/certs/ca-certificates.crt ldapwhoami -x -ZZ -H "$L" -D "$T" -y <(printf %s "$LDAP_PW_TESTER"))")"
check "name outside SANs rejected (hostname check)" rejected "$(verdict "$(rc ldapwhoami -x -ZZ -H ldap://localhost:3389 -D "$T" -y <(printf %s "$LDAP_PW_TESTER"))")"

echo "== authentication still works (over StartTLS) =="
check "tester can bind"                    0  "$(rc as "$T" "$LDAP_PW_TESTER" ldapwhoami)"
check "svc-keycloak can bind"              0  "$(rc as "$K" "$LDAP_PW_SVC_KEYCLOAK" ldapwhoami)"
check "svc-gateway can bind"               0  "$(rc as "$G" "$LDAP_PW_SVC_GATEWAY" ldapwhoami)"
check "wrong password rejected (49)"       49 "$(rc as "$T" "definitely-wrong" ldapwhoami)"

echo "== password hashes are unreadable =="
check "tester cannot read yasaswin hash"   0  "$(srch "$T" "$LDAP_PW_TESTER" -b "$Y" userPassword | count '^userPassword')"
check "tester cannot read OWN hash"        0  "$(srch "$T" "$LDAP_PW_TESTER" -b "$T" userPassword | count '^userPassword')"
check "svc-gateway cannot read any hash"   0  "$(srch "$G" "$LDAP_PW_SVC_GATEWAY" -b "ou=people,$B" userPassword | count '^userPassword')"
check "svc-keycloak cannot read any hash"  0  "$(srch "$K" "$LDAP_PW_SVC_KEYCLOAK" -b "ou=people,$B" userPassword | count '^userPassword')"

echo "== least-privilege visibility =="
check "tester sees own entry"              1  "$(srch "$T" "$LDAP_PW_TESTER" -b "$T" -s base uid | count '^dn:')"
check "tester cannot see yasaswin entry"   0  "$(srch "$T" "$LDAP_PW_TESTER" -b "$Y" -s base uid | count '^dn:')"
check "tester cannot enumerate groups"     0  "$(srch "$T" "$LDAP_PW_TESTER" -b "ou=groups,$B" '(objectClass=groupOfNames)' cn | count '^dn:')"
check "svc-keycloak sees both people"      2  "$(srch "$K" "$LDAP_PW_SVC_KEYCLOAK" -b "ou=people,$B" '(objectClass=inetOrgPerson)' uid | count '^dn: uid=')"
check "svc-gateway reads groot-admins members" 1 "$(srch "$G" "$LDAP_PW_SVC_GATEWAY" -b "cn=groot-admins,ou=groups,$B" -s base member | count '^member:')"
check "svc-keycloak cannot enumerate services" 0 "$(srch "$K" "$LDAP_PW_SVC_KEYCLOAK" -b "ou=services,$B" '(objectClass=*)' cn | count '^dn:')"

echo "== no write paths =="
check "tester cannot join groot-admins (50)" 50 "$(rc as "$T" "$LDAP_PW_TESTER" ldapmodify < <(printf 'dn: cn=groot-admins,ou=groups,%s\nchangetype: modify\nadd: member\nmember: %s\n' "$B" "$T"))"
check "svc-gateway cannot modify a user (50)" 50 "$(rc as "$G" "$LDAP_PW_SVC_GATEWAY" ldapmodify < <(printf 'dn: %s\nchangetype: modify\nreplace: mail\nmail: pwned@evil\n' "$Y"))"

echo
echo "$pass passed, $fail failed"
exit $(( fail > 0 ))
