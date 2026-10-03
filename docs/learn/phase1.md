# Phase 1 — JWT Validation Core

## What We Built

Two packages and a test suite. No running service yet — this is pure library
code that the gateway (Phase 5) will import.

### iam/pkg/jwks/jwks.go
JWKS cache. One public method: GetKey(kid) returns the RSA or EC public key
for that key ID, fetching from the IdP's JWKS endpoint if needed.

Key design choices:
- Double-checked locking: RLock on the hot path (cache hit), Lock only on miss.
  This keeps concurrent JWT validation at read-lock cost only.
- Rate-limited refetch: a minimum interval (default 5 minutes) between JWKS
  fetches. Without this, an attacker flooding unknown kid values would trigger
  an HTTP call to Keycloak on every request.
- Supports RSA (RS256) and EC (ES256) key types. Both are decoded from
  base64url-encoded big integers using only the standard library.

### iam/pkg/token/token.go
JWT validator. One public method: Validate(tokenStr) returns verified Claims
or an error.

Key design choices:
- Algorithm allow-list: RS256 and ES256 only. Checked in keyFunc before any
  cryptography runs. Defeats alg=none and HS256 confusion attacks.
- ClockSkewLeeway: 30 seconds, matching the OIDC spec recommendation.
- Issuer and audience checked against server-side lists, not the token's own
  claims.
- jti denylist: in-memory map with lazy pruning. Adequate for single-process;
  production would use Redis.

### iam/pkg/token/token_test.go
17 tests, all table-driven, all run with go test -race. The test JWKS server
is a real httptest.Server — the cache code makes real HTTP calls during tests.

## Why Each Decision

### Why algorithm allow-list instead of block-list?
A block-list requires knowing every bad algorithm in advance. An allow-list
is closed by default — anything not explicitly permitted is rejected. When
the jwt library adds a new algorithm in the future, our validator rejects it
until we explicitly add it. This is the principle of least privilege applied
to cryptography.

### Why check alg in keyFunc, not before ParseWithClaims?
keyFunc receives the parsed-but-unverified token. The jwt library calls it
as part of verification. Returning an error from keyFunc causes ParseWithClaims
to return that error — the signature is never checked. This is the correct
interception point because the algorithm is read from the header (attacker-
controlled) and we must reject it before trusting any header field.

### Why double-checked locking in the JWKS cache?
The common case is a cache hit. If we took a write lock on every request, all
concurrent JWT validations would serialize. Double-checked locking lets cache
hits proceed with a read lock (many goroutines can hold read locks simultaneously)
and only serializes on cache misses (rare: once per key per MinRefetch interval).

### Why in-memory jti denylist instead of Redis?
We are building incrementally. An in-memory denylist is correct, testable, and
sufficient for a single-process gateway. The trade-off (revocations lost on
restart, no cross-replica revocation) is documented in LIMITATIONS.md.
Phase 2 will add Keycloak session management which handles the common logout case.

### Why lazy pruning of the denylist instead of a background goroutine?
A background goroutine needs its own lifecycle management (context cancellation,
shutdown, testing hooks). Lazy pruning (check expiry on every isRevoked call,
delete if expired) achieves the same bounded memory with no extra goroutines.
It is slightly less prompt about freeing memory but correct and simple.

## Trade-offs

| Decision | Chosen | Rejected | Why |
|----------|--------|----------|-----|
| JWT library | golang-jwt/jwt/v5 | lestrrat-go/jwx | Simpler; writing our own JWKS cache teaches more |
| JWKS parsing | stdlib only | jose library | 10 lines of big.Int math; no dependency needed |
| jti store | in-memory map | Redis | Sufficient for Phase 1; Redis added when needed |
| Test JWKS | httptest.Server | mock/interface | Real HTTP call exercises the full cache code path |
| Key types | RSA + EC | RSA only | ES256 is increasingly common; costs 15 extra lines |

## 5 Interview Questions

**Q1: Explain the alg=none attack and how your code prevents it.**
Some JWT libraries read the algorithm from the token header and use it to
select the verification method. An attacker crafts a header with "alg":"none"
and omits the signature. A naive library calls Verify with no key and accepts
the token. Our keyFunc checks token.Method.Alg() against an explicit allow-list
before returning any key. If alg is not RS256 or ES256, keyFunc returns an
error and ParseWithClaims rejects the token — no cryptography runs at all.

**Q2: What is the HS256 confusion attack?**
RS256 uses asymmetric keys: Keycloak signs with a private key, the gateway
verifies with the public key. HS256 uses a symmetric key: the same secret
signs and verifies. The attack: take the RS256 public key (which is public —
available at the JWKS endpoint) and use it as the HS256 HMAC secret to forge
a token. A library that trusts the header's alg field would verify this forged
token successfully using the public key as the HMAC secret. Our allow-list
rejects HS256 before verification, so the attack has no effect.

**Q3: Why does the JWKS cache use double-checked locking?**
JWT validation is on the hot path — every request goes through it. If we held
a write lock for every validation, all requests would serialize. Double-checked
locking lets the common case (cache hit) proceed under a read lock, which
allows unlimited concurrent readers. We only take the write lock on a cache
miss, which is rare (once per new kid, rate-limited). The double-check after
acquiring the write lock prevents two goroutines that both saw a miss from
both fetching simultaneously.

**Q4: What is the clock-skew leeway and why 30 seconds?**
JWT exp claims are Unix timestamps. If Keycloak's clock and the gateway's
clock differ by even one second, a token that expires at T would fail
validation at T+1 on the gateway even though Keycloak considers it valid.
A leeway window accepts tokens that expired within the window. 30 seconds
is the OIDC core specification's recommendation — enough to cover NTP drift
on typical servers without meaningfully extending token lifetime.

**Q5: How does your jti denylist work and what are its limits?**
RevokeJTI adds a jti and its expiry time to an in-memory map under a write
lock. isRevoked does a read lock lookup, then checks if the expiry has passed
(in which case the token was invalid anyway and we delete the entry lazily).
Limits: the denylist is lost on process restart — a revoked token becomes
valid again after restart until it expires naturally. It also does not work
across multiple gateway replicas. Production fix: back the denylist with Redis
using SETEX (set with expiry) so entries are shared and durable.

## Known Limitations

- jti denylist is in-memory; does not survive restart or work across replicas.
  Documented in docs/learn/LIMITATIONS.md.
- No service yet: this is library code only. The HTTP gateway that uses it
  comes in Phase 5.
- ES256 key parsing is implemented but no ES256 tests exist yet (all tests
  use RS256). Added in Phase 4 when we set up the private CA.

## What Is Next

Phase 2: OIDC and OAuth 2.0 flows.
- Keycloak in Docker (realm "groot", client "groot-gateway")
- Go backend-for-frontend: Authorization Code + PKCE for dashboard login
- Client Credentials flow for n8n service-to-service
- Secure/HttpOnly/SameSite=Strict cookie management
- Tests: state mismatch, nonce mismatch, replayed code, expired refresh
