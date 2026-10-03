package gateway_test

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
	gw "github.com/yasaswinpalukuri/groot-iam/gateway"
)

// ── Test fixtures ─────────────────────────────────────────────────────────────

const (
	testIssuer   = "http://keycloak:8090/realms/groot"
	testAudience = "groot-gateway"
	testSubject  = "test-user-sub-123"
)

var (
	testPrivKey *rsa.PrivateKey
	testKid     = "proxy-test-kid"
)

func TestMain(m *testing.M) {
	var err error
	testPrivKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("generate test RSA key: " + err.Error())
	}
	m.Run()
}

// ── Test backend: records headers received from the proxy ─────────────────────

// captureBackend records the headers of every request it receives.
// This is how we prove the proxy's header stripping/injection works.
type captureBackend struct {
	mu      sync.Mutex
	headers []http.Header // one entry per request received
}

func (b *captureBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	b.headers = append(b.headers, r.Header.Clone())
	b.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (b *captureBackend) lastHeaders() http.Header {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.headers) == 0 {
		return nil
	}
	return b.headers[len(b.headers)-1]
}

func (b *captureBackend) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.headers)
}

// ── Mock JWKS server ──────────────────────────────────────────────────────────

func newJWKSServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": testKid,
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(testPrivKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(testPrivKey.PublicKey.E)).Bytes()),
				},
			},
		})
	}))
}

// ── Token factory ─────────────────────────────────────────────────────────────

type tokenOpts struct {
	expDelta time.Duration
	roles    []string
	subject  string
}

func makeToken(t *testing.T, opts tokenOpts) string {
	t.Helper()
	if opts.expDelta == 0 {
		opts.expDelta = 10 * time.Minute
	}
	if opts.subject == "" {
		opts.subject = testSubject
	}
	roles := opts.roles
	if roles == nil {
		roles = []string{"agent_access"}
	}

	claims := jwt.MapClaims{
		"iss":          testIssuer,
		"aud":          []string{testAudience},
		"sub":          opts.subject,
		"exp":          time.Now().Add(opts.expDelta).Unix(),
		"iat":          time.Now().Unix(),
		"realm_access": map[string]interface{}{"roles": roles},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = testKid
	signed, err := tok.SignedString(testPrivKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// ── Proxy factory ─────────────────────────────────────────────────────────────

func newTestProxy(t *testing.T, backendURL, jwksURL string) *gw.Proxy {
	t.Helper()
	p, err := gw.New(gw.Config{
		BackendURL:       backendURL,
		JWKSUrl:          jwksURL,
		AllowedIssuers:   []string{testIssuer},
		AllowedAudiences: []string{testAudience},
		MaxBodyBytes:     1024, // 1KB for tests
		Routes:           grootTestRoutes,
	})
	if err != nil {
		t.Fatalf("New proxy: %v", err)
	}
	// Allow immediate JWKS refetch in tests
	p.Validator().Cache().MinRefetch = 0
	return p
}

// ── Header injection prevention tests — the most important tests ──────────────

// TestStripsInjectedUserHeaderWithValidToken is the core security test.
//
// Scenario: an attacker adds "X-Verified-User: admin" to their request,
// hoping the backend trusts it. They also have a valid JWT (for a non-admin user).
//
// Expected: the backend sees X-Verified-User set to the JWT subject (not "admin").
func TestStripsInjectedUserHeaderWithValidToken(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Verified-User", "admin") // attacker injection
	req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{}))

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if backend.callCount() == 0 {
		t.Fatal("backend was not called")
	}

	got := backend.lastHeaders().Get("X-Verified-User")
	if got == "admin" {
		t.Error("SECURITY FAILURE: injected X-Verified-User 'admin' reached the backend")
	}
	if got != testSubject {
		t.Errorf("backend X-Verified-User = %q, want %q", got, testSubject)
	}
}

// TestStripsInjectedUserHeaderWithNoToken verifies that even when auth fails,
// the injected header is NOT forwarded. Backend should never be called.
func TestStripsInjectedUserHeaderWithNoToken(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Verified-User", "admin") // attacker injection, no token
	// No Authorization header

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if backend.callCount() > 0 {
		t.Error("backend must NOT be called when auth fails")
	}
}

// TestStripsAllIdentityHeaders verifies every identity header is stripped.
func TestStripsAllIdentityHeaders(t *testing.T) {
	headers := []string{
		"X-Verified-User",
		"X-Verified-Roles",
		"X-User",
		"X-Remote-User",
		"X-Auth-User",
		"X-Forwarded-User",
		"X-Role",
		"X-Roles",
	}

	for _, hdr := range headers {
		hdr := hdr
		t.Run(hdr, func(t *testing.T) {
			backend := &captureBackend{}
			backendSrv := httptest.NewServer(backend)
			defer backendSrv.Close()

			jwksSrv := newJWKSServer(t)
			defer jwksSrv.Close()

			proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(hdr, "injected-value")
			req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{}))

			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			got := backend.lastHeaders().Get(hdr)
			// For X-Verified-User and X-Verified-Roles, the proxy re-injects them
			// with real values — check they're not the injected value
			if hdr == "X-Verified-User" {
				if got == "injected-value" {
					t.Errorf("SECURITY: injected %s reached backend as 'injected-value'", hdr)
				}
			} else if got == "injected-value" {
				t.Errorf("SECURITY: injected %s='injected-value' reached backend", hdr)
			}
		})
	}
}

// ── Auth enforcement tests ────────────────────────────────────────────────────

// TestNoTokenReturns401 verifies unauthenticated requests are rejected.
func TestNoTokenReturns401(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if backend.callCount() > 0 {
		t.Error("backend must not be called with no token")
	}
}

// TestExpiredTokenReturns401 verifies expired tokens are rejected.
func TestExpiredTokenReturns401(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{expDelta: -2 * time.Minute}))

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestValidTokenForwardedToBackend verifies happy path: token valid → backend called.
func TestValidTokenForwardedToBackend(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{
		roles:   []string{"agent_access", "agent:einstein"},
		subject: "yasaswin-sub",
	}))

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if backend.callCount() == 0 {
		t.Fatal("backend should be called with valid token")
	}

	// Verify injected headers
	h := backend.lastHeaders()
	if h.Get("X-Verified-User") != "yasaswin-sub" {
		t.Errorf("X-Verified-User = %q, want %q", h.Get("X-Verified-User"), "yasaswin-sub")
	}
	roles := h.Get("X-Verified-Roles")
	if !strings.Contains(roles, "agent_access") {
		t.Errorf("X-Verified-Roles %q missing agent_access", roles)
	}
	if !strings.Contains(roles, "agent:einstein") {
		t.Errorf("X-Verified-Roles %q missing agent:einstein", roles)
	}
}

// TestSessionCookieToken verifies that the session cookie (set by OIDC callback)
// is also accepted as a token source.
func TestSessionCookieToken(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "groot_session",
		Value: makeToken(t, tokenOpts{}),
	})

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with session cookie, got %d", w.Code)
	}
}

// ── Security header tests ─────────────────────────────────────────────────────

// TestSecurityHeadersOnSuccess verifies security headers are present on 200.
func TestSecurityHeadersOnSuccess(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{}))

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assertSecurityHeaders(t, w)
}

// TestSecurityHeadersOnUnauthorized verifies security headers are ALSO present on 401.
// Security headers on error responses are as important as on success responses.
func TestSecurityHeadersOnUnauthorized(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	req := httptest.NewRequest("GET", "/", nil)
	// No token → 401
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	assertSecurityHeaders(t, w)
}

// TestBodySizeLimitEnforced verifies oversized bodies are rejected.
func TestBodySizeLimitEnforced(t *testing.T) {
	backend := &captureBackend{}
	backendSrv := httptest.NewServer(backend)
	defer backendSrv.Close()

	jwksSrv := newJWKSServer(t)
	defer jwksSrv.Close()

	proxy := newTestProxy(t, backendSrv.URL, jwksSrv.URL)

	// Send Content-Length > 1KB (our test limit)
	req := httptest.NewRequest("POST", "/api/run", strings.NewReader(strings.Repeat("x", 2048)))
	req.Header.Set("Authorization", "Bearer "+makeToken(t, tokenOpts{}))
	req.ContentLength = 2048

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

// ── Helper ────────────────────────────────────────────────────────────────────

func assertSecurityHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	required := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for k, want := range required {
		if got := w.Header().Get(k); got != want {
			t.Errorf("security header %s = %q, want %q", k, got, want)
		}
	}
	// HSTS must be present (value checked separately since it may vary)
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("Strict-Transport-Security header missing")
	}
}
