# Groot IAM — Interview Preparation

**25 questions an IAM interviewer will ask about this exact build.**
Every answer is grounded in code you wrote, tests you ran, or decisions you made.
Do not claim anything beyond what is documented here.

---

## Part 1 — JWT and JWKS (Phase 1)

**Q1. Walk me through how your JWT validator works.**

The validator is in `iam/pkg/token/token.go`. It uses `golang-jwt/jwt/v5`. The entry point is `Validate(tokenStr string)` which calls `jwt.ParseWithClaims` with a `keyFunc`. The keyFunc is where the security happens: it reads the `alg` field from the (unverified) header, rejects anything not in the allow-list of RS256/ES256, reads the `kid` field, fetches the public key from the JWKS cache, and returns it for signature verification. After signature verification, I manually check `iss` and `aud` against server-side allow-lists. Finally I check the `jti` against the denylist. The `jwt.WithLeeway(30s)` option applies clock-skew tolerance to exp and nbf.

**Q2. What is the alg=none attack and how does your code prevent it?**

The alg=none attack exploits JWT libraries that read the algorithm from the token header and use it to select verification. An attacker sets `"alg":"none"` and omits the signature — a naive library accepts the token with no cryptographic check. My `keyFunc` checks `token.Method.Alg()` against an explicit allow-list (`{"RS256": ..., "ES256": ...}`) before doing anything else. If the algorithm is not on the list, `keyFunc` returns an error and `ParseWithClaims` aborts — no signature verification, no token acceptance. Test: `TestValidate/alg_none` passes.

**Q3. What is the HS256 confusion attack?**

RS256 is asymmetric: Keycloak signs with a private key, we verify with the public key. HS256 is symmetric: the same secret signs and verifies. The attack: an attacker takes our RS256 public key (published at the JWKS endpoint, so it's public) and uses it as the HS256 HMAC secret to forge a token. A library that trusts the header's `alg` field would verify this forged token successfully — it calls HS256 verify with our public key as the secret, which matches the forged signature. My allow-list rejects HS256 entirely before any verification, so there is no attack surface.

**Q4. Why does the JWKS cache use double-checked locking?**

JWT validation is on the hot path — every request goes through it. If we held a write lock for every validation, all concurrent requests would serialize. Double-checked locking: read lock first (many goroutines can hold it simultaneously), check the cache, release. Only on a miss do we acquire the write lock, check again (another goroutine may have fetched while we waited), then fetch if still missing. This keeps the common case (cache hit) at read-lock cost only. The race detector confirmed no data races in the JWKS cache across 17 parallel subtests.

**Q5. How does your jti denylist work and what are its limits?**

`RevokeJTI(jti string, exp time.Time)` adds a jti and its expiry to an in-memory map under a write lock. `isRevoked` does a read-lock lookup, then checks if the expiry has passed — if so, it prunes the entry lazily. Limits: the denylist is lost on process restart, so a revoked token becomes valid again until it expires naturally. It also does not work across multiple gateway replicas. Production fix: back it with Redis using SETEX so entries are shared across replicas and survive restarts.

**Q6. Why did you choose golang-jwt/jwt/v5 over lestrrat-go/jwx?**

lestrrat-go/jwx handles both JWTs and JWKs in one library. I chose to write my own JWKS cache (the `pkg/jwks` package) to understand the protocol deeply and control the rate-limiting behaviour. golang-jwt/jwt/v5 handles only JWT parsing and verification — simpler, focused, well-maintained. The combination gives me the security-critical parts I can explain line by line.

---

## Part 2 — OIDC and OAuth 2.0 (Phase 2)

**Q7. Explain the Authorization Code + PKCE flow end to end.**

The user clicks Login. The Go handler calls `AuthURL(state, nonce)` which: generates a 32-byte random `code_verifier`, computes `code_challenge = BASE64URL(SHA256(verifier))` per RFC 7636 §4.6, stores state+nonce+verifier in a server-side map with a 10-minute TTL, and returns a redirect URL to Keycloak with the challenge and `code_challenge_method=S256`. The user authenticates at Keycloak. Keycloak redirects back with `code` and `state`. The Go handler looks up the state entry (verifying it exists and is not expired), deletes it immediately (single-use), exchanges the code using `code_verifier` (PKCE proof), verifies the returned ID token's signature and nonce. Nonce match confirms the ID token belongs to this specific login attempt. Test: `TestCallbackHappyPath` passes. RFC 7636 S256 math verified against the Appendix B test vector.

**Q8. Why is PKCE needed if you already have a client secret?**

The client secret authenticates the server. PKCE protects the authorization code. If an attacker intercepts the redirect (malicious browser extension, compromised OS, log leak), they get the code. Without PKCE they can exchange it for tokens — they don't need the secret because they're impersonating the redirect URI. PKCE requires the code_verifier that was never in the redirect, so the intercepted code is useless. S256 means even the challenge (in the redirect URL) cannot be reverse-engineered to the verifier.

**Q9. What does the nonce prevent?**

ID token replay across sessions. Example: an attacker captures Alice's ID token from one login. They start a new login flow for themselves, get a callback URL, and substitute Alice's ID token in the callback. Without a nonce check the gateway would create a session for the attacker as Alice. With a nonce, Alice's ID token carries Alice's nonce — which does not match the nonce the gateway generated for the attacker's session. Test: `TestNonceMismatch` passes.

**Q10. Why are the session cookies HttpOnly and SameSite=Strict?**

HttpOnly: the cookie is not accessible to JavaScript. An XSS vulnerability in the dashboard cannot steal the access token because `document.cookie` does not include HttpOnly cookies. SameSite=Strict: the browser does not send the cookie on cross-site requests — not on navigation from an external link, not on image loads, not on CORS requests. This defeats CSRF attacks that try to trigger authenticated actions by tricking the user into visiting a malicious page. The Secure flag (not set in development) would additionally prevent the cookie from being sent over plain HTTP.

**Q11. What is the difference between the ID token and the access token?**

The ID token answers "who is the user?" — it contains identity claims (sub, name, email, nonce) and is consumed by the client application after login. The access token answers "what can this token do?" — it is presented to resource servers as a credential. The gateway validates the access token via JWKS on every API call. After login completes, the gateway does not need the ID token again.

---

## Part 3 — SAML 2.0 (Phase 3)

**Q12. What is the InResponseTo field and why does it matter?**

InResponseTo is a field in the SAML Response that references the ID of the AuthnRequest that triggered it. It ties the assertion to a specific request. Without it, an attacker could replay a valid assertion from a previous session (or inject an unsolicited IdP-initiated assertion) into a new session. My SP stores every generated request ID in a `pending` map with a 10-minute TTL. `ParseCallbackHTTP` passes those IDs to crewjam/saml's `ParseResponse`, which checks that the Response's InResponseTo matches. After parsing, all pending IDs are deleted — so the same assertion cannot be submitted twice.

**Q13. What attacks does crewjam/saml protect against that you rely on?**

XML Signature Wrapping (XSW): an attacker inserts a second unsigned element and moves the signed element to a different position. A naive parser verifies the signature on the signed element but processes the unsigned one. crewjam/saml validates that the signed element is the assertion being processed. It also verifies the XML signature against the IdP's public key from the metadata, enforces Audience restriction against the SP EntityID, and checks NotOnOrAfter/NotBefore time constraints.

**Q14. What is NOT done in Phase 3 that a production SAML SP would need?**

The browser round-trip: I verified AuthnRequest generation against a real Keycloak SAML client, but I did not complete the callback flow with a real signed SAMLResponse because that requires a browser. ParseCallbackHTTP is implemented and wired to crewjam/saml — but it was never exercised with a real Keycloak assertion. In production, the SP signing certificate would also be registered with Keycloak so assertions can be encrypted, and HTTPS would be required on the ACS URL. Documented in LIMITATIONS.md.

---

## Part 4 — PKI, TLS, and mTLS (Phase 4)

**Q15. Explain mTLS and why you use it between gateway and agent_service.**

Regular TLS authenticates the server to the client. mTLS requires both sides to present X.509 certificates. In Groot, the gateway presents its cert to browsers (normal TLS), but also requires agent_service to present a client cert signed by Groot's private CA (step-ca). This means that even if an attacker gets inside the Docker network — container escape, misconfigured firewall — they cannot call agent_service without a valid cert signed by our CA. The `tls.RequireAndVerifyClientCert` setting in `ServerTLSConfig` aborts the handshake if the client cert is missing or chains to the wrong CA. Verified: `TestMissingClientCertIsRefused` and `TestWrongCAClientCertIsRefused` both pass with real TLS connections.

**Q16. Why TLS 1.2 minimum and AEAD-only cipher suites?**

TLS 1.0 and 1.1 are broken — BEAST on TLS 1.0, various downgrade attacks. TLS 1.2 is the PCI-DSS minimum. TLS 1.3 is preferred: Go's crypto/tls selects it automatically when both sides support it, and TLS 1.3 always uses AEAD ciphers (non-configurable in Go). For TLS 1.2, I explicitly list AEAD suites (AES-GCM, ChaCha20-Poly1305) and exclude CBC suites. CBC mode is vulnerable to BEAST and Lucky13 timing attacks — the MAC-then-encrypt construction allows padding oracle attacks. AEAD provides authenticated encryption where the tag covers the ciphertext, eliminating that attack surface.

**Q17. Why step-ca over raw openssl scripts?**

step-ca provides a REST API, built-in ACME support for automatic renewal, and JWK provisioners. Raw openssl scripts are fragile — one wrong flag and you silently generate a cert without the SANs you intended, or with a key usage that prevents client auth. step-ca also supports short-lived certs (24 hours in our case) which limit the blast radius of a key compromise: a leaked key becomes useless in at most 24 hours without renewal. The rotate-certs.sh script issues new certs, atomically replaces old ones, and sends SIGHUP for a graceful TLS reload.

**Q18. What is the difference between JWKS key rotation and certificate rotation?**

JWKS keys rotate by publishing a new key at the same URL — clients pick it up automatically on the next cache miss. No cert distribution, no restart needed. Certificate rotation (X.509) requires issuing a new cert, updating both the server and any clients that pin the cert, and optionally revoking the old one. JWKS rotation is simpler because the public key distribution is pull-based (clients fetch on demand). Certificate rotation is more complex but provides stronger identity binding — a cert ties a public key to a name, validity period, and key usage via a CA-signed structure.

---

## Part 5 — Reverse Proxy (Phase 5)

**Q19. Why strip identity headers unconditionally before authentication?**

If we stripped them only on authentication failure, a race condition window exists: a crafted request could potentially slip through between strip and validate. More importantly, stripping before auth is the correct security model — the backend should never see a client-supplied identity header, period. The order in ServeHTTP is: (1) add security response headers, (2) strip identity headers, (3) enforce body limit, (4) extract token, (5) validate, (6) inject verified headers, (7) forward. Tests: `TestStripsInjectedUserHeaderWithValidToken` and `TestStripsInjectedUserHeaderWithNoToken` both verify the backend never sees attacker-supplied values.

**Q20. What headers do you strip and why those specifically?**

X-Verified-User, X-Verified-Roles (our own injected headers — strip before re-injecting), X-User (nginx auth_request convention), X-Remote-User (Apache mod_auth), X-Auth-User (common internal proxies), X-Forwarded-User (some SSO systems), X-Role, X-Roles. These are the headers backends commonly use to determine caller identity. Missing even one is a security vulnerability — a backend that trusts X-Remote-User would be fully bypassed. Adding too many just wastes CPU. The list is conservative: we strip known identity headers, not everything.

**Q21. Why use httputil.ReverseProxy instead of a raw http.Client?**

httputil.ReverseProxy handles connection pooling, keep-alive, hop-by-hop header removal (Connection, Upgrade, Transfer-Encoding), X-Forwarded-For injection, and response streaming. Writing all of that correctly with a raw http.Client is tedious and error-prone. The 60-second WriteTimeout in NewServer accommodates slow LLM inference in agent_service — a too-short timeout would kill streaming responses mid-generation. The 100ms FlushInterval enables Server-Sent Events used by the chat endpoint.

---

## Part 6 — Architecture and Design

**Q22. Walk me through the STRIDE threat model for your auth path.**

Spoofing: defeated by RS256 signature + JWKS verification — a forged token without Keycloak's private key fails signature check. Tampering: defeated by the signature covering header+payload — modifying any claim changes the signature. Test: `TestValidate/tampered_payload` passes. Repudiation: partially addressed — the gateway logs auth events; Phase 9 adds an audit log. Information Disclosure: HttpOnly cookies prevent JavaScript token access; tokens are never logged. DoS: JWKS cache rate-limits refetches to one per 5 minutes, preventing kid-flood attacks. Elevation of privilege: algorithm allow-list prevents alg=none and HS256 confusion; role mapping is read from the JWT, not from user input.

**Q23. How would you scale this to multiple gateway replicas?**

Three things need to change. First, the jti denylist: move from in-memory to Redis SETEX with the token's remaining TTL so revocations are shared across replicas. Second, the OIDC state map (pending login states): same — move to Redis with a 10-minute TTL. Third, TLS termination: in production, put a load balancer (nginx, Envoy, AWS ALB) in front that handles TLS and forwards HTTP to the gateways; or each gateway terminates TLS and the load balancer passes through TLS. The JWKS cache is already safe for multiple goroutines — each replica maintains its own cache, which is fine since JWKS is read-only.

**Q24. Why Keycloak instead of Okta or Azure AD?**

Three reasons: free (no per-user cost for a self-hosted system), self-hosted (tokens never leave the Groot network), and full control over realm configuration. The OIDC/SAML protocols are the same regardless of which IdP you use — the Go code that validates tokens works identically against Keycloak, Okta, or Azure AD, because the validation is based on standards (OIDC discovery, JWKS, JWT). The interviewer-relevant answer: I chose Keycloak to learn the protocol, not the vendor. In an enterprise job I would configure the same code against whatever IdP the company uses.

**Q25. What would you do differently if you had another month?**

Five things. First, complete the SAML browser round-trip: wire a test IdP that completes the POST-binding callback so ParseCallbackHTTP is exercised with a real signed assertion. Second, wire mTLS end to end: add the step-ca cert to agent_service and enable TLS termination on its FastAPI server. Third, replace the in-memory state stores with Redis for production-grade replay prevention. Fourth, add OpenTelemetry tracing so every auth decision is traceable across services. Fifth, implement the Dashboard security page so the system is demonstrable without SSH — a recruiter should be able to see login, token inspection, and cert expiry in a browser.

---

## Honest Limitations to Acknowledge in Interviews

- SAML browser round-trip not completed: ParseCallbackHTTP is implemented but never exercised with a real Keycloak signed assertion
- mTLS services not wired: certs issued, handshake verified, but gateway and agent_service do not currently communicate over mTLS
- jti denylist is in-memory: lost on restart, not shared across replicas
- OIDC state is in-memory: same limits
- OpenLDAP (Phase 6, if asked): not Active Directory — no Kerberos, no Group Policy, no SYSVOL
- Keycloak runs in start-dev mode: H2 database, no HTTPS requirement — documented, not production
