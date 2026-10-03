package token_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yasaswinpalukuri/groot-iam/pkg/token"
)

// ── Test fixtures ────────────────────────────────────────────────────────────

const (
	testIssuer   = "http://keycloak:8090/realms/groot"
	testAudience = "groot-gateway"
)

// testKeys holds the RSA key pairs generated once for the entire test run.
// Using package-level variables avoids re-generating 2048-bit RSA keys for
// every subtest (expensive). TestMain initialises them.
var (
	key1Priv *rsa.PrivateKey // primary signing key
	key2Priv *rsa.PrivateKey // second key for rotation tests

	jwksMu      sync.Mutex // guards jwksKeys
	jwksKeys    []map[string]interface{}
	jwksServer  *httptest.Server
)

// TestMain runs before all tests in this package.
// It generates key pairs, starts the JWKS server, and tears down after.
func TestMain(m *testing.M) {
	var err error
	key1Priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("generate key1: " + err.Error())
	}
	key2Priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("generate key2: " + err.Error())
	}

	// Start with key1 in the JWKS
	jwksKeys = []map[string]interface{}{rsaKeyToJWK("kid-1", &key1Priv.PublicKey)}

	jwksServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwksMu.Lock()
		keys := make([]map[string]interface{}, len(jwksKeys))
		copy(keys, jwksKeys)
		jwksMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"keys": keys})
	}))
	defer jwksServer.Close()

	m.Run()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// newValidator creates a Validator pointing at the test JWKS server.
// MinRefetch is set to 0 so tests can trigger refetches without waiting.
func newValidator() *token.Validator {
	v := token.NewValidator(jwksServer.URL, []string{testIssuer}, []string{testAudience})
	v.Cache().MinRefetch = 0
	return v
}

// makeToken creates a signed JWT with the given parameters.
// expDelta: duration from now for exp (negative = already expired).
// nbfDelta: duration from now for nbf (positive = not yet valid).
func makeToken(
	t *testing.T,
	priv *rsa.PrivateKey,
	kid, iss, aud string,
	expDelta, nbfDelta time.Duration,
	jti string,
	roles []string,
) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": iss,
		"aud": []string{aud},
		"sub": "yasaswin",
		"exp": now.Add(expDelta).Unix(),
		"iat": now.Unix(),
		"realm_access": map[string]interface{}{
			"roles": roles,
		},
	}
	if nbfDelta != 0 {
		claims["nbf"] = now.Add(nbfDelta).Unix()
	}
	if jti != "" {
		claims["jti"] = jti
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// rsaKeyToJWK serialises an RSA public key to the JWK JSON map.
func rsaKeyToJWK(kid string, pub *rsa.PublicKey) map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// setJWKSKeys atomically replaces the keys served by the test JWKS server.
func setJWKSKeys(keys []map[string]interface{}) {
	jwksMu.Lock()
	jwksKeys = keys
	jwksMu.Unlock()
}

// tamperPayload decodes a JWT's payload, applies fn to the claims map,
// and reassembles the token. The signature is left intact, so the result
// is a syntactically valid but cryptographically invalid JWT.
func tamperPayload(t *testing.T, tokenStr string, fn func(map[string]interface{})) string {
	t.Helper()
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatal("tamperPayload: not a 3-part JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("tamperPayload decode: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("tamperPayload unmarshal: %v", err)
	}
	fn(claims)
	modified, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("tamperPayload marshal: %v", err)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString(modified)
	return strings.Join(parts, ".")
}

// algNoneToken crafts a JWT with alg=none and an empty signature.
// The golang-jwt library won't sign with none, so we build it manually.
// This tests that our allow-list rejects it before any crypto runs.
func algNoneToken(iss, aud string) string {
	header := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"alg":"none","typ":"JWT"}`),
	)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{
		"sub":"attacker",
		"iss":"` + iss + `",
		"aud":["` + aud + `"],
		"exp":9999999999,
		"iat":1000000000
	}`))
	return header + "." + payload + "." // empty signature
}

// ── Table-driven validation tests ────────────────────────────────────────────

func TestValidate(t *testing.T) {
	// Reset JWKS to key1 only before running subtests
	setJWKSKeys([]map[string]interface{}{rsaKeyToJWK("kid-1", &key1Priv.PublicKey)})

	v := newValidator()

	validToken := makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
		10*time.Minute, 0, "jti-valid-1", []string{"agent:groot"})

	cases := []struct {
		name        string
		token       func() string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid_token",
			token:   func() string { return validToken },
			wantErr: false,
		},
		{
			name: "expired",
			token: func() string {
				return makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
					-2*time.Minute, 0, "", nil)
			},
			wantErr:     true,
			errContains: "token is expired",
		},
		{
			name: "not_yet_valid_beyond_leeway",
			token: func() string {
				// Sign a real token with nbf 2 min in future — beyond 30s leeway
				return makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
					10*time.Minute, 2*time.Minute, "", nil)
			},
			wantErr:     true,
			errContains: "not valid yet",
		},
		{
			name: "clock_skew_within_leeway",
			// exp 20 seconds ago — within the 30-second leeway, should pass
			token: func() string {
				return makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
					-20*time.Second, 0, "", nil)
			},
			wantErr: false,
		},
		{
			name: "clock_skew_outside_leeway",
			// exp 60 seconds ago — beyond the 30-second leeway, should fail
			token: func() string {
				return makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
					-60*time.Second, 0, "", nil)
			},
			wantErr:     true,
			errContains: "token is expired",
		},
		{
			name: "wrong_issuer",
			token: func() string {
				return makeToken(t, key1Priv, "kid-1", "https://evil.example.com", testAudience,
					10*time.Minute, 0, "", nil)
			},
			wantErr:     true,
			errContains: "issuer",
		},
		{
			name: "wrong_audience",
			token: func() string {
				return makeToken(t, key1Priv, "kid-1", testIssuer, "other-service",
					10*time.Minute, 0, "", nil)
			},
			wantErr:     true,
			errContains: "audience",
		},
		{
			name:        "alg_none",
			token:       func() string { return algNoneToken(testIssuer, testAudience) },
			wantErr:     true,
			errContains: "algorithm",
		},
		{
			name: "tampered_payload",
			token: func() string {
				// Take a valid token, elevate roles in the payload, leave signature intact
				return tamperPayload(t, validToken, func(c map[string]interface{}) {
					c["realm_access"] = map[string]interface{}{
						"roles": []string{"admin", "superuser"},
					}
				})
			},
			wantErr: true,
			// Signature verification catches the tamper
		},
		{
			name: "missing_kid",
			token: func() string {
				// Build a token without kid in the header
				claims := jwt.MapClaims{
					"iss": testIssuer,
					"aud": []string{testAudience},
					"sub": "test",
					"exp": time.Now().Add(10 * time.Minute).Unix(),
					"iat": time.Now().Unix(),
				}
				tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				// kid is NOT set in the header
				delete(tok.Header, "kid")
				signed, err := tok.SignedString(key1Priv)
				if err != nil {
					t.Fatalf("sign no-kid token: %v", err)
				}
				return signed
			},
			wantErr:     true,
			errContains: "kid",
		},
		{
			name:        "malformed_token",
			token:       func() string { return "not.a.valid.jwt.at.all" },
			wantErr:     true,
		},
		{
			name:        "empty_token",
			token:       func() string { return "" },
			wantErr:     true,
			errContains: "empty",
		},
		{
			name: "revoked_jti",
			token: func() string {
				tok := makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
					10*time.Minute, 0, "jti-revoked-99", nil)
				// Revoke the jti before validating
				v.RevokeJTI("jti-revoked-99", time.Now().Add(10*time.Minute))
				return tok
			},
			wantErr:     true,
			errContains: "revoked",
		},
	}

	for _, tc := range cases {
		tc := tc // capture range variable for parallel subtests
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			claims, err := v.Validate(tc.token())
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got claims: %+v", claims)
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if claims == nil {
					t.Error("expected claims, got nil")
				}
			}
		})
	}
}

// TestKeyRotation verifies that the gateway handles key rotation mid-flight:
// a new kid appears in the JWKS and is accepted after a cache refetch.
func TestKeyRotation(t *testing.T) {
	// Start with only key1
	setJWKSKeys([]map[string]interface{}{rsaKeyToJWK("kid-1", &key1Priv.PublicKey)})

	v := newValidator()
	// MinRefetch=0 allows immediate refetch on unknown kid

	// Token signed with key1 works
	tok1 := makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
		10*time.Minute, 0, "", nil)
	if _, err := v.Validate(tok1); err != nil {
		t.Fatalf("key1 token should validate: %v", err)
	}

	// Rotate: IdP now serves key2 (key1 is gone — simulates full rotation)
	setJWKSKeys([]map[string]interface{}{rsaKeyToJWK("kid-2", &key2Priv.PublicKey)})

	// Token signed with key2 should now validate (cache refetches on unknown kid)
	tok2 := makeToken(t, key2Priv, "kid-2", testIssuer, testAudience,
		10*time.Minute, 0, "", nil)
	if _, err := v.Validate(tok2); err != nil {
		t.Fatalf("key2 token should validate after rotation: %v", err)
	}

	// Old token with kid-1 should now fail (key1 no longer in JWKS)
	if _, err := v.Validate(tok1); err == nil {
		t.Error("kid-1 token should fail after key1 removed from JWKS")
	}
}

// TestRateLimitedRefetch verifies that a second unknown-kid request within
// MinRefetch is rejected without making another HTTP call.
func TestRateLimitedRefetch(t *testing.T) {
	setJWKSKeys([]map[string]interface{}{rsaKeyToJWK("kid-1", &key1Priv.PublicKey)})

	v := newValidator()
	v.Cache().MinRefetch = 10 * time.Minute // rate-limit to 10 minutes

	// First request: kid-1 not cached, triggers a fetch, populates cache
	tok1 := makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
		10*time.Minute, 0, "", nil)
	if _, err := v.Validate(tok1); err != nil {
		t.Fatalf("first validation should pass: %v", err)
	}

	// Now request an unknown kid-99 — rate limit prevents a new fetch
	tok99 := makeToken(t, key2Priv, "kid-99", testIssuer, testAudience,
		10*time.Minute, 0, "", nil)
	_, err := v.Validate(tok99)
	if err == nil {
		t.Fatal("unknown kid within rate limit should fail")
	}
	if !strings.Contains(err.Error(), "rate-limited") {
		t.Errorf("expected rate-limited error, got: %v", err)
	}
}

// TestRolesAndHasRole verifies that Claims.Roles() and HasRole() work.
func TestRolesAndHasRole(t *testing.T) {
	setJWKSKeys([]map[string]interface{}{rsaKeyToJWK("kid-1", &key1Priv.PublicKey)})
	v := newValidator()

	tok := makeToken(t, key1Priv, "kid-1", testIssuer, testAudience,
		10*time.Minute, 0, "", []string{"agent:groot", "agent:einstein"})

	claims, err := v.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !claims.HasRole("agent:groot") {
		t.Error("expected role agent:groot")
	}
	if !claims.HasRole("agent:einstein") {
		t.Error("expected role agent:einstein")
	}
	if claims.HasRole("admin") {
		t.Error("should not have role admin")
	}
	if len(claims.Roles()) != 2 {
		t.Errorf("expected 2 roles, got %d", len(claims.Roles()))
	}
}
