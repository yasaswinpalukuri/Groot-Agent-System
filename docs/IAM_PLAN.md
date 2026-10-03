# Groot IAM - Architecture Plan

## ASCII Architecture

    Browser / Telegram / n8n
            |
            v
    +------------------------------------------+
    |         IAM Gateway (Go, :8080/:8443)    |
    |  - TLS termination                       |
    |  - JWT validation (JWKS, RS256/ES256)    |
    |  - OIDC session (Authorization Code)    |
    |  - SAML SP                               |
    |  - Header stripping + injection          |
    |  - Role to agent_access mapping          |
    +------------------+-----------------------+
                       | mTLS + X-Verified-User
            +----------+----------+
            v                     v
      agent_service          other services
      (FastAPI :8000)        (n8n, dashboard)
            |
            v
      ChromaDB / Ollama

    Identity Providers (Docker, groot-iam network):
      Keycloak   :8090  (OIDC + SAML IdP)
      OpenLDAP   :8389  (directory, NOT Active Directory)

## Services, Ports, RAM Cost

| Service     | Port(s)    | RAM est. | Phase |
|-------------|------------|----------|-------|
| iam-gateway | 8080, 8443 | ~20 MB   | 1-5   |
| keycloak    | 8090       | ~512 MB  | 2     |
| openldap    | 8389, 8636 | ~50 MB   | 6     |
| step-ca     | 9000       | ~30 MB   | 4     |

Total new RAM: ~612 MB peak. Available: 14 GB. Safe.

## Data Flows

### Dashboard Login (OIDC Authorization Code + PKCE)
1. User hits dashboard -> gateway redirects to Keycloak
2. User authenticates at Keycloak
3. Keycloak redirects back with auth code
4. Gateway exchanges code for ID token + access token
5. Gateway sets Secure/HttpOnly/SameSite=Strict cookie
6. Gateway injects X-Verified-User, X-Verified-Roles headers
7. agent_service trusts only those headers (strips client-supplied)

### Service-to-Service (Client Credentials)
1. n8n sends client_id + client_secret to Keycloak token endpoint
2. Keycloak returns access token (JWT)
3. n8n includes Bearer token in requests to gateway
4. Gateway validates JWT against JWKS, checks iss/aud/exp/roles
5. Gateway forwards to agent_service with verified headers

### mTLS (Phase 4+)
1. Gateway presents server cert (signed by Groot CA)
2. agent_service presents client cert (signed by Groot CA)
3. Both verify the other cert chain
4. No password, no token needed between them

## STRIDE Threat Model

| Threat        | Example                     | Control                           |
|---------------|-----------------------------|-----------------------------------|
| Spoofing      | Forged JWT claims           | RS256 sig + JWKS verify           |
| Tampering     | Modified payload            | Signature covers header+payload   |
| Repudiation   | Deny calling an endpoint    | Audit log (Phase 9)               |
| Info Disclose | Token in URL or logs        | HttpOnly cookie, no token in log  |
| DoS           | kid-flood JWKS storm        | JWKS cache + rate-limited refetch |
| Elevation     | alg=none or HS256 confusion | Algorithm allow-list RS256/ES256  |

## Limitations
See docs/learn/LIMITATIONS.md
