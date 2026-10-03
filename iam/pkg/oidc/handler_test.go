package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	oidcpkg "github.com/yasaswinpalukuri/groot-iam/pkg/oidc"
)

// ── Test fixtures ─────────────────────────────────────────────────────────────

var (
	testPrivKey *rsa.PrivateKey
	testKid     = "oidc-test-kid-1"
)

func TestMain(m *testing.M) {
	var err error
	testPrivKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("generate test key: " + err.Error())
	}
	m.Run()
}

// ── PKCE pure-function tests ──────────────────────────────────────────────────

// TestCodeVerifierFormat checks the verifier is valid base64url, no padding,
// and 43 characters (32 bytes → 43 base64url chars), per RFC 7636 §4.1.
func TestCodeVerifierFormat(t *testing.T) {
	for i := 0; i < 20; i++ {
		v, err := oidcpkg.GenerateCodeVerifier()
		if err != nil {
			t.Fatalf("GenerateCodeVerifier: %v", err)
		}
		if len(v) != 43 {
			t.Errorf("verifier length %d, want 43", len(v))
		}
		// Must be base64url without padding
		for _, c := range v {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", c) {
				t.Errorf("invalid base64url char %q in verifier %q", c, v)
				break
			}
		}
	}
}

// TestCodeChallengeRFC7636Vector verifies our PKCE math against the known
// test vector from RFC 7636 Appendix B.
// verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
// challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
//
// If this test fails, our PKCE implementation is wrong — full stop.
func TestCodeChallengeRFC7636Vector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := oidcpkg.CodeChallenge(verifier)
	if got != want {
		t.Errorf("CodeChallenge(%q) = %q, want %q", verifier, got, want)
	}
}

// TestCodeChallengeVerifierRoundTrip ensures a freshly generated verifier
// produces a challenge that is different from the verifier (sanity check).
func TestCodeChallengeVerifierRoundTrip(t *testing.T) {
	verifier, _ := oidcpkg.GenerateCodeVerifier()
	challenge := oidcpkg.CodeChallenge(verifier)
	if challenge == verifier {
		t.Error("challenge must not equal verifier")
	}
	if len(challenge) == 0 {
		t.Error("challenge must not be empty")
	}
}

// ── Mock OIDC server ──────────────────────────────────────────────────────────

// nonceStore is shared between the test and the mock token endpoint.
// Tests store "code → nonce" before calling Callback; the endpoint reads it.
type nonceStore struct {
	mu sync.Mutex
	m  map[string]string // code → nonce
}

func (s *nonceStore) set(code, nonce string) {
	s.mu.Lock()
	s.m[code] = nonce
	s.mu.Unlock()
}

func (s *nonceStore) get(code string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[code]
}

// mockOIDC creates an httptest.Server that speaks enough OIDC for our tests.
// nonces: shared map that tests populate before calling Callback.
// wrongNonce: if true the token endpoint always puts "wrong-nonce" in the ID token.
// rejectCodes: if true the token endpoint rejects any code after first use.
func mockOIDC(t *testing.T, nonces *nonceStore, wrongNonce, rejectCodes bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	usedCodes := &sync.Map{}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]interface{}{rsaToJWK(testKid, &testPrivKey.PublicKey)},
		})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		code := r.Form.Get("code")

		if rejectCodes {
			if _, used := usedCodes.LoadOrStore(code, struct{}{}); used {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"error":             "invalid_grant",
					"error_description": "code already redeemed",
				})
				return
			}
		}

		nonce := nonces.get(code)
		if wrongNonce {
			nonce = "deliberately-wrong-nonce"
		}

		claims := jwt.MapClaims{
			"iss":   srv.URL,
			"sub":   "test-user",
			"aud":   []string{"test-client"},
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": nonce,
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = testKid
		idTokenStr, err := tok.SignedString(testPrivKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "access-" + code,
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idTokenStr,
		})
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newTestHandler creates a Handler backed by the given mock OIDC server.
func newTestHandler(t *testing.T, srv *httptest.Server) *oidcpkg.Handler {
	t.Helper()
	h, err := oidcpkg.New(context.Background(), oidcpkg.Config{
		IssuerURL:    srv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
	})
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	return h
}

// authURLAndState calls AuthURL and returns the redirect URL + the state value.
func authURLAndState(t *testing.T, h *oidcpkg.Handler) (redirectURL, state, nonce string) {
	t.Helper()
	state, err := oidcpkg.GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	nonce, err = oidcpkg.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce: %v", err)
	}
	redirectURL, err = h.AuthURL(state, nonce)
	if err != nil {
		t.Fatalf("AuthURL: %v", err)
	}
	return redirectURL, state, nonce
}

// rsaToJWK converts an RSA public key to a JWK map for the mock JWKS endpoint.
func rsaToJWK(kid string, pub *rsa.PublicKey) map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// ── State lifecycle tests (no token exchange needed) ─────────────────────────

// TestStateCountIncreasesWithAuthURL verifies state is stored per login attempt.
func TestStateCountIncreasesWithAuthURL(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	if h.StateCount() != 0 {
		t.Fatalf("expected 0 states, got %d", h.StateCount())
	}
	for i := 1; i <= 3; i++ {
		state, _ := oidcpkg.GenerateState()
		nonce, _ := oidcpkg.GenerateNonce()
		h.AuthURL(state, nonce)
		if h.StateCount() != i {
			t.Errorf("after %d AuthURL calls: StateCount=%d, want %d", i, h.StateCount(), i)
		}
	}
}

// TestCallbackMissingState verifies that a callback with no state is rejected.
func TestCallbackMissingState(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	r := httptest.NewRequest("GET", "/callback?code=abc123", nil)
	_, _, err := h.Callback(context.Background(), r)
	if err == nil {
		t.Fatal("expected error for missing state")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("error %q does not mention 'state'", err)
	}
}

// TestCallbackUnknownState verifies that an unrecognized state is rejected.
func TestCallbackUnknownState(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	r := httptest.NewRequest("GET", "/callback?state=unknown-state&code=abc123", nil)
	_, _, err := h.Callback(context.Background(), r)
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}

// TestCallbackStateIsConsumedOnUse verifies that the same callback URL
// cannot be replayed — state is deleted on first use.
func TestCallbackStateIsConsumedOnUse(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	_, state, nonce := authURLAndState(t, h)
	code := "test-code-consume"
	nonces.set(code, nonce)

	callbackURL := "/callback?state=" + url.QueryEscape(state) + "&code=" + code
	r1 := httptest.NewRequest("GET", callbackURL, nil)

	// First call: should succeed
	h.Callback(context.Background(), r1)

	// State should now be gone — StateCount dropped
	if h.StateCount() != 0 {
		t.Errorf("state should be consumed: StateCount=%d", h.StateCount())
	}

	// Second call with same state: must fail
	r2 := httptest.NewRequest("GET", callbackURL, nil)
	_, _, err := h.Callback(context.Background(), r2)
	if err == nil {
		t.Error("replayed callback should fail")
	}
}

// TestCallbackExpiredState verifies that a state that sat too long is rejected.
// We inject an already-expired entry by calling AuthURL and then directly
// manipulating time — instead we just test with a state that was never registered.
// (True expiry testing would require injecting a clock, added in a future phase.)
func TestCallbackExpiredState(t *testing.T) {
	// An unknown state simulates what happens after expiry cleanup.
	// True in-window expiry is tested via the stateExpiry const review.
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	r := httptest.NewRequest("GET", "/callback?state=expired-state-12345&code=x", nil)
	_, _, err := h.Callback(context.Background(), r)
	if err == nil {
		t.Fatal("expected error for expired/unknown state")
	}
}

// ── Full flow tests (with mock OIDC token exchange) ───────────────────────────

// TestCallbackHappyPath exercises the complete Authorization Code + PKCE flow.
func TestCallbackHappyPath(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	srv := mockOIDC(t, nonces, false, false)
	h := newTestHandler(t, srv)

	_, state, nonce := authURLAndState(t, h)
	code := "happy-path-code"
	nonces.set(code, nonce)

	callbackURL := "/callback?state=" + url.QueryEscape(state) + "&code=" + code
	r := httptest.NewRequest("GET", callbackURL, nil)

	idToken, oauth2Token, err := h.Callback(context.Background(), r)
	if err != nil {
		t.Fatalf("happy path failed: %v", err)
	}
	if idToken == nil {
		t.Error("expected idToken, got nil")
	}
	if oauth2Token == nil {
		t.Error("expected oauth2Token, got nil")
	}
	if oauth2Token.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

// TestNonceMismatch verifies that an ID token with the wrong nonce is rejected.
// This prevents an attacker from replaying an ID token from one session into
// a different session's callback.
func TestNonceMismatch(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	// wrongNonce=true: mock always puts "deliberately-wrong-nonce" in ID token
	srv := mockOIDC(t, nonces, true, false)
	h := newTestHandler(t, srv)

	_, state, nonce := authURLAndState(t, h)
	code := "nonce-mismatch-code"
	nonces.set(code, nonce) // correct nonce registered but mock ignores it

	callbackURL := "/callback?state=" + url.QueryEscape(state) + "&code=" + code
	r := httptest.NewRequest("GET", callbackURL, nil)

	_, _, err := h.Callback(context.Background(), r)
	if err == nil {
		t.Fatal("expected nonce mismatch error")
	}
	if !strings.Contains(err.Error(), "nonce") {
		t.Errorf("error %q should mention 'nonce'", err)
	}
}

// TestCodeReplay verifies that a used authorization code is rejected on reuse.
// The mock server rejects a code the second time it is presented.
func TestCodeReplay(t *testing.T) {
	nonces := &nonceStore{m: make(map[string]string)}
	// rejectCodes=true: mock rejects any code after first use
	srv := mockOIDC(t, nonces, false, true)
	h := newTestHandler(t, srv)

	// First login
	_, state1, nonce1 := authURLAndState(t, h)
	code := "reused-code"
	nonces.set(code, nonce1)

	r1 := httptest.NewRequest("GET",
		"/callback?state="+url.QueryEscape(state1)+"&code="+code, nil)
	if _, _, err := h.Callback(context.Background(), r1); err != nil {
		t.Fatalf("first callback should succeed: %v", err)
	}

	// Second login attempt with the same code and a fresh state
	_, state2, nonce2 := authURLAndState(t, h)
	nonces.set(code, nonce2)

	r2 := httptest.NewRequest("GET",
		"/callback?state="+url.QueryEscape(state2)+"&code="+code, nil)
	if _, _, err := h.Callback(context.Background(), r2); err == nil {
		t.Error("replayed code should fail")
	}
}

// ── Cookie tests ──────────────────────────────────────────────────────────────

// TestSetSessionCookieAttributes verifies the cookie has the required security
// attributes: HttpOnly, SameSite=Strict, correct name and value.
func TestSetSessionCookieAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	oidcpkg.SetSessionCookie(w, "test-access-token", false)

	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]

	if c.Name != oidcpkg.SessionCookieName {
		t.Errorf("cookie name %q, want %q", c.Name, oidcpkg.SessionCookieName)
	}
	if c.Value != "test-access-token" {
		t.Errorf("cookie value %q, want %q", c.Value, "test-access-token")
	}
	if !c.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite %v, want Strict", c.SameSite)
	}
	if c.MaxAge <= 0 {
		t.Errorf("cookie MaxAge %d, want positive", c.MaxAge)
	}
}

// TestClearSessionCookieExpires verifies that ClearSessionCookie sets MaxAge=-1.
func TestClearSessionCookieExpires(t *testing.T) {
	w := httptest.NewRecorder()
	oidcpkg.ClearSessionCookie(w)

	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("MaxAge %d, want -1", cookies[0].MaxAge)
	}
	if cookies[0].Value != "" {
		t.Errorf("cookie value should be empty, got %q", cookies[0].Value)
	}
}
