# Phase 0 — Recon and Plan

## What We Built

No service code yet. This phase is all setup and understanding.

1. **Go 1.22.5** installed from the official tarball at /usr/local/go. Checksum
   verified before extraction. Not from apt — apt lags 2+ major versions behind
   and could silently downgrade crypto/tls on an apt upgrade.

2. **git worktree** at ~/code/groot-iam on branch feature/iam. The live code
   at ~/code/tony stays on main, unaffected. Both share one .git — no duplicate
   history, no risk of committing half-finished IAM code to main.

3. **.gitignore** written before any code. Rules cover *.pem, *.key, .env,
   secrets/ and Go build artifacts. The rule: ignore secrets before they exist,
   not after.

4. **Go module** initialized as github.com/yasaswinpalukuri/groot-iam. The full
   GitHub path is the module identity — correct for potential publishing and
   what interviewers expect.

5. **docs/IAM_PLAN.md** — architecture, ports, RAM budget, three data flows
   (OIDC login, Client Credentials, mTLS), STRIDE model.

6. **docs/learn/LIMITATIONS.md** — honest record started. OpenLDAP is not AD.
   Keycloak is not Okta. mTLS is not yet built. Nothing goes on the resume
   until tests pass.

7. **Backup** — local copy, pendrive copy, git tag on GitHub. Three independent
   copies before any changes to the live stack.

## Why Each Decision

**Why RS256/ES256 only (algorithm allow-list)?**
JWT libraries historically accepted whatever algorithm the token header claimed.
An attacker could send a token with alg=none (no signature at all) and some
libraries would accept it. The HS256 confusion attack takes the IdP's public
key and uses it as an HMAC secret. Both attacks are defeated by an explicit
allow-list that rejects everything except RS256 and ES256 before any
verification happens.

**Why JWKS cache with rate-limited refetch?**
If you refetch the JWKS every time you see an unknown kid, an attacker can
flood you with tokens carrying random kid values, each forcing an outbound
HTTP call to Keycloak. A cache with a minimum refetch interval (e.g. 5 minutes)
caps the blast at one HTTP call per interval regardless of token volume.

**Why both OIDC and SAML?**
Modern apps use OIDC. Enterprise customers frequently have SAML-only IdPs
(ADFS, Okta in SAML mode, PingFederate). Supporting both makes Groot
enterprise-ready without replacing the OIDC path.

**Why a private CA instead of Let's Encrypt?**
Let's Encrypt requires a public DNS name. Groot is internal, reached over
Tailscale. A private CA (step-ca) issues certs trusted within the Groot
network for mTLS between gateway and agent_service. The trade-off: the CA
cert must be distributed to every client that needs to trust it. Documented
in LIMITATIONS.md.

## Trade-offs

| Decision | Chosen | Rejected | Why |
|----------|--------|----------|-----|
| Go install | Official tarball | apt | apt version lags, can downgrade crypto |
| IAM isolation | git worktree | branch switch | main stays live and testable |
| IdP | Keycloak | Okta/Azure AD | Free, self-hosted, full OIDC+SAML |
| Directory | OpenLDAP | Samba AD | RAM cost; Samba AD needs ~1GB+ |
| CA | step-ca | openssl scripts | step-ca has built-in ACME and rotation |

## 5 Likely Interview Questions

**Q1: Why did you choose Go for the IAM gateway?**
Go's standard library has production-grade TLS, HTTP/2, and crypto primitives.
The net/http server is concurrency-safe by default. Compiled binaries ship
without a runtime dependency — one binary, no Python virtualenv or JVM to
manage. For a gateway that must handle auth on every request, latency and
correctness matter more than developer convenience.

**Q2: What is the difference between OIDC and OAuth 2.0?**
OAuth 2.0 is a delegation framework — it answers "can this app access this
resource on behalf of this user?" It says nothing about who the user is.
OIDC adds an identity layer: it defines a standard JWT format called the
ID token and a /userinfo endpoint. The ID token says who you are; the access
token says what you can do. You cannot use a plain OAuth 2.0 access token
for login because it carries no guaranteed identity claims.

**Q3: What is the alg=none attack?**
Some JWT libraries parsed the algorithm from the token header and used it
to select the verification method. An attacker crafts a token with
"alg": "none" and omits the signature entirely. A naive library skips
verification. The fix: never read the algorithm from the token. Maintain
an explicit server-side allow-list (RS256, ES256) and reject anything else
before touching the signature.

**Q4: Why does the JWKS cache need rate-limiting on refetches?**
Each cache miss triggers an HTTP call to Keycloak's JWKS endpoint. An
attacker who can forge token headers can set arbitrary kid values, causing
a cache miss on every request. Without rate-limiting, this becomes a DoS
against both Groot and Keycloak. A 5-minute minimum between refetches caps
the damage regardless of request rate.

**Q5: What is mTLS and how does it differ from regular TLS?**
Regular TLS authenticates the server to the client — the browser verifies
the server's certificate. mTLS requires both sides to present certificates.
In Groot, the gateway presents its server cert to browsers (normal TLS),
but also requires agent_service to present a client cert signed by Groot's
private CA. This means that even if someone gets inside the Docker network,
they cannot call agent_service without a valid cert. No password, no token,
no API key — the cert is the credential.

## Known Limitations

- OpenLDAP is not Active Directory. No Kerberos, no Group Policy.
- Keycloak is not Okta or Azure AD.
- mTLS, audit log, and SAML not yet implemented.
- step-ca certs trusted only within Groot network.

## What Is Next

Phase 1: Go JWT validation core.
Files: iam/pkg/token/token.go, iam/pkg/jwks/jwks.go, iam/pkg/token/token_test.go
Tests: 12+ table-driven cases with go test -race covering expired, wrong issuer,
wrong audience, alg=none, tampered payload, unknown kid, key rotation,
clock skew, malformed token, revoked jti.
