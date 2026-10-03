# Phase 2 — OIDC and OAuth 2.0 Flows

## What We Built

### Keycloak 24.0.5 in Docker
- Compose project at ~/code/groot-iam/keycloak/
- Port 8090 (127.0.0.1 only — not publicly reachable)
- Named network groot-iam (shared with gateway in Phase 5)
- Realm "groot" with two clients and one user

### Keycloak realm configuration
- Client groot-gateway: Authorization Code + PKCE (S256 enforced)
- Client n8n-service: Client Credentials only (service accounts enabled)
- User yasaswin: test user for dashboard login
- Direct access grants disabled on groot-gateway after smoke test

### iam/pkg/oidc/handler.go
OIDC handler. Key methods:
- AuthURL(state, nonce): registers PKCE verifier + nonce in server-side map,
  returns redirect URL with code_challenge
- Callback(ctx, r): validates state (single-use, TTL), exchanges code with
  code_verifier, verifies ID token signature and nonce
- SetSessionCookie/ClearSessionCookie: HttpOnly, SameSite=Strict
- ClientCredentialsToken: service-to-service token fetch

### iam/pkg/oidc/handler_test.go
13 tests, all pass with go test -race:
- RFC 7636 test vector (PKCE math verified against the spec)
- State lifecycle (creation, consumption, unknown state)
- Full callback happy path (mock OIDC server)
- Nonce mismatch rejection
- Code replay rejection
- Cookie security attributes

### Smoke test
Phase 1 JWT validator connected to real Keycloak JWKS and validated a
real RS256 token for user yasaswin. Subject, issuer, and roles all correct.

## Why Each Decision

### Why PKCE and why S256 not Plain?
PKCE prevents an attacker who intercepts the authorization code redirect from
exchanging it for tokens. The code_verifier never travels through the browser
— only the code_challenge (its SHA256 hash) is in the authorization URL.
Plain PKCE sends the verifier as the challenge, so intercepting the redirect
gives the attacker everything. S256 is mandatory; we enforce it by setting
pkce.code.challenge.method=S256 on the Keycloak client, so the IdP rejects
any auth request without a valid challenge.

### Why store state server-side?
The state parameter prevents CSRF: the callback must present the same state
value that was in the authorization URL. If we stored it only in a cookie,
an attacker could forge a callback with a crafted state that matches a
forged cookie. Server-side storage means the gateway is the authority on
which states are valid — the browser cannot influence that.

### Why delete state on first use?
The callback URL (containing state and code) may appear in browser history,
server logs, or referrer headers. If state were reusable, replaying the URL
would re-trigger the code exchange. Deleting state on first read means the
second attempt gets "unknown state" — the callback URL becomes useless after
one use.

### Why check nonce after code exchange?
The ID token nonce prevents an attacker from replaying a valid ID token from
one user's session into another user's callback. Example: attacker intercepts
Alice's ID token (e.g. via a compromised log). They start a new login flow,
get a callback URL, and substitute Alice's ID token. Without a nonce check,
the gateway would accept it and log the attacker in as Alice. With a nonce
check, Alice's token carries Alice's nonce — which does not match the nonce
the gateway generated for the attacker's session.

### Why HttpOnly + SameSite=Strict cookies?
HttpOnly: JavaScript cannot read the cookie. An XSS vulnerability in the
dashboard cannot steal the access token because document.cookie does not
include HttpOnly cookies.
SameSite=Strict: the browser does not send the cookie on any cross-site
request, including navigation from an external link. This defeats CSRF
attacks that try to trigger authenticated actions by tricking the user into
visiting a malicious page.

### Why Client Credentials for n8n?
n8n is a machine — it has no user to authenticate. The Client Credentials
grant issues a short-lived access token (default 5 minutes in Keycloak)
in exchange for a client_id + client_secret. The gateway validates this
token via JWKS — same path as user tokens. No separate auth logic needed.
Rotating the n8n client secret invalidates future tokens immediately.

### Why start-dev for Keycloak?
start-dev removes the HTTPS requirement and uses an embedded H2 database.
This is acceptable for development. Production would use start with a
PostgreSQL database and TLS configured. Documented in LIMITATIONS.md.

## Trade-offs

| Decision | Chosen | Rejected | Why |
|----------|--------|----------|-----|
| State storage | In-memory map | Cookie | Server is authority; browser cannot forge |
| PKCE method | S256 | Plain | Plain provides zero protection |
| Session cookie | Access token | Encrypted session ID | Simpler; JWT is self-contained |
| Keycloak mode | start-dev | start + PostgreSQL | Phase 4 adds TLS and production config |
| Nonce | Per-login random | Omit | Prevents ID token replay across sessions |

## 5 Interview Questions

**Q1: What is PKCE and why is it needed for the Authorization Code flow?**
PKCE (Proof Key for Code Exchange, RFC 7636) prevents authorization code
interception attacks. In the Authorization Code flow, the IdP redirects the
browser to your callback URL with a code in the URL. If an attacker intercepts
that redirect (via a malicious app, browser extension, or log), they get the
code. Without PKCE they can exchange it for tokens. With PKCE, the client
generates a random code_verifier, sends its SHA256 hash (code_challenge) in
the authorization request, and proves ownership by including the verifier in
the code exchange request. The attacker who only has the code cannot produce
the verifier, so the exchange fails.

**Q2: What is the difference between the ID token and the access token?**
The ID token answers "who is the user?" — it contains identity claims (sub,
name, email, nonce) and is consumed by the client application. The access
token answers "what can this token do?" — it is presented to resource servers
(like our gateway) as a credential. The gateway validates the access token
via JWKS; it does not need the ID token after login. In OIDC, you always get
both from a code exchange, but they serve different purposes.

**Q3: Why do you delete the state entry immediately when the callback arrives,
before completing the code exchange?**
Two reasons. First, if the code exchange fails (network error, Keycloak down),
the user must start a new login — the old callback URL is no longer valid. This
is intentional: a failed exchange should not leave a reusable state entry that
an attacker could exploit. Second, if two requests arrive with the same state
simultaneously (race condition), only the first one finds the entry; the second
gets "unknown state." This prevents a race-condition replay where two concurrent
callbacks both succeed.

**Q4: What would you change to make this production-ready?**
Four things: (1) TLS on Keycloak and the gateway — done in Phase 4. (2)
PostgreSQL instead of H2 for Keycloak — H2 is single-process only. (3)
Distribute state storage to Redis so multiple gateway replicas share state —
currently state is in-memory per process. (4) Encrypt the session cookie —
currently it carries the raw access token; encrypting it with AES-GCM hides
claims from the browser and lets the server invalidate sessions independently
of token expiry.

**Q5: How does Client Credentials differ from Authorization Code, and when
do you use each?**
Authorization Code is for delegated access on behalf of a human user. The
user authenticates at the IdP, consents, and the code is exchanged for tokens
that represent that user. Client Credentials is for machine-to-machine auth
where there is no user. The service authenticates with its own client_id and
client_secret, and the resulting token represents the service, not a user.
In Groot: the dashboard uses Authorization Code + PKCE (human logs in). n8n
uses Client Credentials (workflow engine calls the gateway as itself).

## Known Limitations (added to LIMITATIONS.md)
- Keycloak runs in start-dev mode: no TLS, H2 database, hostname not strict
- State is in-memory: does not survive gateway restart, not shared across replicas
- Session cookie carries raw access token: not encrypted at rest
- Direct access grants were temporarily enabled for smoke testing; disabled after

## What Is Next
Phase 3: SAML 2.0 SP using crewjam/saml against a second Keycloak client.
Validate signature, Audience, NotOnOrAfter, InResponseTo, replay prevention.
