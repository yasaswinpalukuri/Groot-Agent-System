// Package token validates JWTs for the Groot IAM gateway.
//
// Design decisions documented here because they will come up in interviews:
//
// 1. Algorithm allow-list (not block-list): we accept ONLY RS256 and ES256.
//    The "alg=none" attack and HS256 confusion attack are both defeated by
//    rejecting everything not on this list — before any cryptography runs.
//
// 2. ClockSkewLeeway: a 30-second window matches the OIDC spec recommendation.
//    Without it, a 1-second clock difference between Keycloak and the gateway
//    would cause valid tokens to fail verification at exactly the exp boundary.
//
// 3. In-memory jti denylist: adequate for a single-process gateway. Production
//    would use Redis so revocation survives restarts and works across replicas.
//    We document this honestly in LIMITATIONS.md.
package token

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yasaswinpalukuri/groot-iam/pkg/jwks"
)

// allowedAlgorithms is the server-side algorithm allow-list.
//
// WHY not read alg from the token header?
// The header is attacker-controlled. Historical vulnerabilities exploited
// libraries that trusted the header's alg:
//   - alg=none: no signature, library skips verification entirely
//   - HS256 confusion: uses the RSA public key as an HMAC secret
// A fixed server-side allow-list defeats both. We pick RS256 and ES256
// because they are asymmetric: the signing key (private) never leaves
// Keycloak, and we only need the public key to verify.
var allowedAlgorithms = map[string]jwt.SigningMethod{
	"RS256": jwt.SigningMethodRS256,
	"ES256": jwt.SigningMethodES256,
}

// ClockSkewLeeway is how much clock drift we tolerate between the gateway
// and Keycloak. 30 seconds matches the OIDC core specification recommendation.
const ClockSkewLeeway = 30 * time.Second

// Claims holds the JWT claims we care about for Groot.
// RegisteredClaims gives us Sub, Iss, Aud, Exp, Nbf, Iat, Jti.
// RealmAccess carries Keycloak's role assignments.
type Claims struct {
	jwt.RegisteredClaims
	// RealmAccess is Keycloak's realm-level role claim.
	// It maps to Groot's agent_access RBAC roles.
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Roles returns the slice of role strings from the token.
func (c *Claims) Roles() []string { return c.RealmAccess.Roles }

// HasRole returns true if the token carries the named role.
// Used by the gateway to enforce Groot's agent_access RBAC.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// denyEntry records a revoked jti with the token's expiry time so we can
// prune the denylist lazily instead of running a background goroutine.
type denyEntry struct {
	exp time.Time
}

// Validator validates JWTs for Groot.
// Create one per application; reuse across requests. It is safe for
// concurrent use.
type Validator struct {
	cache            *jwks.Cache
	allowedIssuers   []string
	allowedAudiences []string

	denyMu      sync.RWMutex
	jtiDenylist map[string]denyEntry
}

// NewValidator creates a Validator.
//
// jwksURL is the IdP's JWKS endpoint, e.g.:
//
//	http://keycloak:8090/realms/groot/protocol/openid-connect/certs
//
// issuers is the list of accepted iss values, e.g.:
//
//	[]string{"http://keycloak:8090/realms/groot"}
//
// audiences is the list of accepted aud values, e.g.:
//
//	[]string{"groot-gateway", "account"}
func NewValidator(jwksURL string, issuers, audiences []string) *Validator {
	return &Validator{
		cache:            jwks.NewCache(jwksURL),
		allowedIssuers:   issuers,
		allowedAudiences: audiences,
		jtiDenylist:      make(map[string]denyEntry),
	}
}

// Cache exposes the underlying JWKS cache so callers can tune MinRefetch
// or inject a test HTTP client.
func (v *Validator) Cache() *jwks.Cache { return v.cache }

// RevokeJTI adds a token ID to the denylist.
// Call this on logout or when a token is compromised.
// exp should be the token's expiry time so the denylist entry self-prunes.
func (v *Validator) RevokeJTI(jti string, exp time.Time) {
	v.denyMu.Lock()
	v.jtiDenylist[jti] = denyEntry{exp: exp}
	v.denyMu.Unlock()
}

// isRevoked checks the denylist, pruning expired entries lazily.
// Lazy pruning avoids a background goroutine while keeping the denylist
// from growing unboundedly (entries expire with their tokens).
func (v *Validator) isRevoked(jti string) bool {
	v.denyMu.RLock()
	entry, found := v.jtiDenylist[jti]
	v.denyMu.RUnlock()
	if !found {
		return false
	}
	if time.Now().After(entry.exp) {
		// Token expired anyway — prune the entry
		v.denyMu.Lock()
		delete(v.jtiDenylist, jti)
		v.denyMu.Unlock()
		return false
	}
	return true
}

// Validate parses and validates a raw JWT string.
// Returns verified Claims on success, or a descriptive error on failure.
//
// Validation order (each step can short-circuit):
//  1. Empty string check
//  2. Parse header — extract alg and kid (still attacker-controlled)
//  3. Algorithm allow-list check — reject alg=none, HS256, anything unknown
//  4. JWKS key lookup by kid (with rate-limited cache refetch)
//  5. Signature verification (cryptographic)
//  6. exp/nbf checks with ClockSkewLeeway
//  7. iss check against allowedIssuers
//  8. aud check against allowedAudiences
//  9. jti denylist check
func (v *Validator) Validate(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, errors.New("token: empty token string")
	}

	var claims Claims

	// jwt.ParseWithClaims calls v.keyFunc for every token.
	// keyFunc is where we enforce the algorithm allow-list and fetch the key.
	// WithLeeway applies ClockSkewLeeway to exp and nbf.
	// WithExpirationRequired rejects tokens that omit exp entirely.
	parsed, err := jwt.ParseWithClaims(
		tokenStr,
		&claims,
		v.keyFunc,
		jwt.WithLeeway(ClockSkewLeeway),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, fmt.Errorf("token: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("token: invalid (unknown reason)")
	}

	// Issuer check — jwt.ParseWithClaims does not validate iss by default.
	// We check it here against our server-side list.
	if err := v.checkIssuer(claims.Issuer); err != nil {
		return nil, err
	}

	// Audience check
	if err := v.checkAudience(claims.Audience); err != nil {
		return nil, err
	}

	// JTI revocation check — only meaningful if the token carries a jti.
	if claims.ID != "" && v.isRevoked(claims.ID) {
		return nil, fmt.Errorf("token: jti %q has been revoked", claims.ID)
	}

	return &claims, nil
}

// keyFunc is called by jwt.ParseWithClaims with the parsed-but-unverified token.
// The token header is attacker-controlled at this point. We must not trust
// alg or kid without explicit validation.
//
// Steps:
//  1. Check alg against the allow-list (defeats alg=none and HS256 confusion)
//  2. Extract kid from the header
//  3. Fetch the public key from the JWKS cache
func (v *Validator) keyFunc(token *jwt.Token) (interface{}, error) {
	// Step 1: algorithm allow-list
	// token.Method is set by the parser based on the header's "alg" field.
	alg := token.Method.Alg()
	signingMethod, allowed := allowedAlgorithms[alg]
	if !allowed {
		// This single check defeats alg=none, HS256 confusion, and any
		// algorithm the jwt library might support that we don't want.
		return nil, fmt.Errorf("token: algorithm %q not allowed (accept: RS256, ES256)", alg)
	}
	// Defense in depth: confirm the method object itself is what we expect.
	if token.Method != signingMethod {
		return nil, fmt.Errorf("token: algorithm method mismatch for %q", alg)
	}

	// Step 2: extract kid
	// The header is a map[string]interface{}. kid must be a non-empty string.
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("token: missing or empty kid in header")
	}

	// Step 3: key lookup
	key, err := v.cache.GetKey(kid)
	if err != nil {
		return nil, fmt.Errorf("token: JWKS key lookup: %w", err)
	}
	return key, nil
}

func (v *Validator) checkIssuer(iss string) error {
	for _, allowed := range v.allowedIssuers {
		if iss == allowed {
			return nil
		}
	}
	return fmt.Errorf("token: issuer %q not in allow-list", iss)
}

func (v *Validator) checkAudience(aud jwt.ClaimStrings) error {
	for _, a := range aud {
		for _, allowed := range v.allowedAudiences {
			if a == allowed {
				return nil
			}
		}
	}
	return fmt.Errorf("token: audience %v not in allow-list", []string(aud))
}
