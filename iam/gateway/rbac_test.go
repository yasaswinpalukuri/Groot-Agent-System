package gateway_test

// Phase 6.7 — gateway authorization (RBAC) tests.
//
// Reuses the fixtures from proxy_test.go (captureBackend, newJWKSServer,
// makeToken, newTestProxy, assertSecurityHeaders, testIssuer/testAudience).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gw "github.com/yasaswinpalukuri/groot-iam/gateway"
)

// grootTestRoutes mirrors Groot's production policy: every authenticated
// route needs agent_access; the /admin subtree additionally needs groot_admin.
// Role names match what Keycloak actually issues (Phase 6.6).
var grootTestRoutes = []gw.RouteRule{
	{PathPrefix: "/", RequireAny: []string{"agent_access"}},
	{PathPrefix: "/admin", RequireAny: []string{"groot_admin"}},
}

type rbacEnv struct {
	backend *captureBackend
	proxy   *gw.Proxy
}

// newRBACEnv builds backend + JWKS + proxy. routes == nil uses newTestProxy
// (the shared factory, which carries grootTestRoutes).
func newRBACEnv(t *testing.T, routes []gw.RouteRule) *rbacEnv {
	t.Helper()
	backend := &captureBackend{}
	bs := httptest.NewServer(backend)
	t.Cleanup(bs.Close)
	js := newJWKSServer(t)
	t.Cleanup(js.Close)

	if routes == nil {
		return &rbacEnv{backend: backend, proxy: newTestProxy(t, bs.URL, js.URL)}
	}
	p, err := gw.New(gw.Config{
		BackendURL:       bs.URL,
		JWKSUrl:          js.URL,
		AllowedIssuers:   []string{testIssuer},
		AllowedAudiences: []string{testAudience},
		MaxBodyBytes:     1024,
		Routes:           routes,
	})
	if err != nil {
		t.Fatalf("New proxy: %v", err)
	}
	p.Validator().Cache().MinRefetch = 0
	return &rbacEnv{backend: backend, proxy: p}
}

func (e *rbacEnv) get(t *testing.T, target, tok string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.proxy.ServeHTTP(w, req)
	return w
}

func agentToken(t *testing.T) string {
	return makeToken(t, tokenOpts{roles: []string{"agent_access"}})
}
func adminToken(t *testing.T) string {
	return makeToken(t, tokenOpts{roles: []string{"agent_access", "groot_admin"}})
}

// ── 403 vs 200 by role ────────────────────────────────────────────────────────

func TestRBACAgentRoleReachesAgentRoutes(t *testing.T) {
	e := newRBACEnv(t, nil)
	w := e.get(t, "/agents/run", agentToken(t))
	if w.Code != http.StatusOK {
		t.Fatalf("agent on /agents/run: got %d, want 200", w.Code)
	}
	if got := e.backend.lastHeaders().Get("X-Verified-Roles"); !strings.Contains(got, "agent_access") {
		t.Fatalf("backend X-Verified-Roles = %q, want it to contain agent_access", got)
	}
}

// The core RBAC test: a VALID token without the role gets 403, and the request
// never reaches the backend (authorization happens AT the gateway).
func TestRBACAgentRoleForbiddenOnAdmin(t *testing.T) {
	e := newRBACEnv(t, nil)
	w := e.get(t, "/admin/users", agentToken(t))
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent on /admin/users: got %d, want 403", w.Code)
	}
	if n := e.backend.callCount(); n != 0 {
		t.Fatalf("backend called %d times on a forbidden request, want 0", n)
	}
	assertSecurityHeaders(t, w)
}

func TestRBACAdminReachesAdminRoutes(t *testing.T) {
	e := newRBACEnv(t, nil)
	if w := e.get(t, "/admin/users", adminToken(t)); w.Code != http.StatusOK {
		t.Fatalf("admin on /admin/users: got %d, want 200", w.Code)
	}
}

func TestRBACNoRolesForbidden(t *testing.T) {
	e := newRBACEnv(t, nil)
	tok := makeToken(t, tokenOpts{roles: []string{}}) // non-nil empty: no default role
	if w := e.get(t, "/agents/run", tok); w.Code != http.StatusForbidden {
		t.Fatalf("roleless token: got %d, want 403", w.Code)
	}
}

// ── Matching semantics ────────────────────────────────────────────────────────

// "/administrator" must NOT be governed by the "/admin" rule.
func TestRBACPrefixMatchesOnSegmentBoundary(t *testing.T) {
	e := newRBACEnv(t, nil)
	if w := e.get(t, "/administrator", agentToken(t)); w.Code != http.StatusOK {
		t.Fatalf("/administrator with agent role: got %d, want 200 (falls under \"/\", not \"/admin\")", w.Code)
	}
	if w := e.get(t, "/admin", agentToken(t)); w.Code != http.StatusForbidden {
		t.Fatalf("/admin exactly with agent role: got %d, want 403", w.Code)
	}
}

// A path no rule covers is denied, even for a valid token with real roles.
func TestRBACDefaultDenyUnmatchedPath(t *testing.T) {
	e := newRBACEnv(t, []gw.RouteRule{{PathPrefix: "/agents", RequireAny: []string{"agent_access"}}})
	if w := e.get(t, "/agents/run", agentToken(t)); w.Code != http.StatusOK {
		t.Fatalf("/agents/run: got %d, want 200", w.Code)
	}
	if w := e.get(t, "/metrics", adminToken(t)); w.Code != http.StatusForbidden {
		t.Fatalf("/metrics (no rule): got %d, want 403 default deny", w.Code)
	}
}

// ── Path-confusion bypass ─────────────────────────────────────────────────────

// Each target would textually match an allowed rule, but a normalising backend
// could serve /admin. Must be refused (400) BEFORE authorization, for any role,
// and never reach the backend.
func TestRBACRejectsNonCanonicalPaths(t *testing.T) {
	cases := []string{
		"/agents/../admin/users",     // dot-dot traversal
		"/agents/%2e%2e/admin/users", // percent-encoded dot-dot
		"//admin/users",              // double slash
		"/agents/./run",              // dot segment
		"/agents//run",               // empty segment
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			e := newRBACEnv(t, nil)
			w := e.get(t, target, adminToken(t)) // even an admin gets 400
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d, want 400", target, w.Code)
			}
			if n := e.backend.callCount(); n != 0 {
				t.Fatalf("%s: backend called %d times, want 0", target, n)
			}
			assertSecurityHeaders(t, w)
		})
	}
	// A single trailing slash is canonical enough and must still work.
	e := newRBACEnv(t, nil)
	if w := e.get(t, "/agents/", agentToken(t)); w.Code != http.StatusOK {
		t.Fatalf("/agents/: got %d, want 200", w.Code)
	}
}

// ── Fail closed at startup ────────────────────────────────────────────────────

func TestNewRejectsUnsafePolicy(t *testing.T) {
	cases := map[string][]gw.RouteRule{
		"empty policy":     nil,
		"empty RequireAny": {{PathPrefix: "/", RequireAny: nil}},
		"relative prefix":  {{PathPrefix: "admin", RequireAny: []string{"groot_admin"}}},
		"trailing slash":   {{PathPrefix: "/admin/", RequireAny: []string{"groot_admin"}}},
		"dot-dot prefix":   {{PathPrefix: "/a/../admin", RequireAny: []string{"groot_admin"}}},
		"duplicate prefix": {{PathPrefix: "/", RequireAny: []string{"a"}}, {PathPrefix: "/", RequireAny: []string{"b"}}},
	}
	for name, routes := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := gw.New(gw.Config{
				BackendURL:       "http://127.0.0.1:1",
				JWKSUrl:          "http://127.0.0.1:1/jwks",
				AllowedIssuers:   []string{testIssuer},
				AllowedAudiences: []string{testAudience},
				Routes:           routes,
			})
			if err == nil {
				t.Fatalf("New accepted an unsafe policy (%s); want error", name)
			}
		})
	}
}

// ── 401 hygiene ───────────────────────────────────────────────────────────────

// The client learns THAT it failed, never WHY (that goes to the server log).
func TestUnauthorizedDoesNotLeakValidatorDetail(t *testing.T) {
	e := newRBACEnv(t, nil)

	w := e.get(t, "/agents/run", makeToken(t, tokenOpts{expDelta: -10 * time.Minute}))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: got %d, want 401", w.Code)
	}
	if body := w.Body.String(); body != "unauthorized\n" {
		t.Fatalf("401 body = %q, want exactly %q (no validator detail)", body, "unauthorized\n")
	}
	if got, want := w.Header().Get("WWW-Authenticate"), `Bearer realm="groot", error="invalid_token"`; got != want {
		t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
	}

	w = e.get(t, "/agents/run", "")
	if got, want := w.Header().Get("WWW-Authenticate"), `Bearer realm="groot"`; got != want {
		t.Fatalf("no-token WWW-Authenticate = %q, want %q", got, want)
	}
}
