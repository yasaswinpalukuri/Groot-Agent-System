// Package gateway implements the Groot IAM reverse proxy.
//
// The proxy sits in front of every Groot service. On each request it:
//  1. Adds security headers to the response (always, even on errors)
//  2. Strips client-supplied identity headers (unconditional — before any auth)
//  3. Rejects non-canonical paths (400) before any routing/authz decision
//  4. Enforces the request body size limit
//  5. Authenticates: extracts and validates the JWT (401 on failure)
//  6. Authorizes: matches the path to a RouteRule and checks roles (403 on failure)
//  7. Injects verified identity headers so backends never do their own auth
//  8. Forwards to the backend via httputil.ReverseProxy
//
// WHY strip before validate, not after?
// If we validated first and only stripped on failure, a race condition could
// let a crafted request slip through with injected headers during the
// validation window. Stripping unconditionally — before any other logic —
// means the backend can never see a client-supplied identity header, period.
//
// WHY 401 vs 403?
// 401 = "I don't know who you are" (no/invalid token): the client should
// re-authenticate. 403 = "I know who you are and you may not do this": re-auth
// won't help. Conflating them breaks clients that refresh tokens on 401.
//
// WHY httputil.ReverseProxy?
// It handles connection pooling, keep-alive, hop-by-hop header removal
// (Connection, Upgrade, etc.), and X-Forwarded-For automatically. Writing
// this from scratch with http.Client is tedious and error-prone.
package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/yasaswinpalukuri/groot-iam/pkg/token"
)

// identityHeaders is the list of headers we strip unconditionally from every
// inbound request. The backend trusts ONLY headers we inject — never client input.
//
// WHY this specific list?
// These are the headers backends commonly use to determine caller identity.
// Missing one is a security bug. Adding too many just wastes CPU.
var identityHeaders = []string{
	"X-Verified-User",  // our own injected header — strip before re-injecting
	"X-Verified-Roles", // our own injected roles header
	"X-User",           // used by nginx auth_request and many frameworks
	"X-Remote-User",    // Apache mod_auth convention
	"X-Auth-User",      // common in internal proxies
	"X-Forwarded-User", // used by some SSO systems (distinct from X-Forwarded-For)
	"X-Role",           // singular role header
	"X-Roles",          // plural role header
}

// securityResponseHeaders are added to every response — including 401/403/400.
// Adding them only on success would mean error pages have weaker security.
var securityResponseHeaders = map[string]string{
	// HSTS: tell browsers to always use HTTPS for 1 year, including subdomains.
	// WHY? Prevents protocol downgrade attacks (SSLstrip).
	// Note: only effective when TLS is active. Harmless on HTTP dev.
	"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	// nosniff: browser must not guess content type from body bytes.
	// WHY? Prevents MIME sniffing attacks where a response with wrong
	// Content-Type is executed as script.
	"X-Content-Type-Options": "nosniff",
	// DENY: page cannot be embedded in a frame.
	// WHY? Prevents clickjacking — an attacker embedding Groot in an invisible iframe.
	"X-Frame-Options": "DENY",
	// Referrer policy: only send origin on cross-site navigation.
	// WHY? Prevents leaking full URLs (which may contain tokens) to third parties.
	"Referrer-Policy": "strict-origin-when-cross-origin",
}

// RFC 6750 §3: a 401 for a bearer-protected resource MUST carry WWW-Authenticate.
// error="invalid_token" tells clients a token WAS presented but rejected
// (refresh or re-login), as opposed to no token at all.
const (
	wwwAuthNoToken      = `Bearer realm="groot"`
	wwwAuthInvalidToken = `Bearer realm="groot", error="invalid_token"`
)

// RouteRule grants access to a path subtree to callers holding ANY of the roles.
//
// Matching is on path-SEGMENT boundaries and the LONGEST prefix wins:
// "/admin" matches "/admin" and "/admin/x", but NOT "/administrator"
// (a naive strings.HasPrefix would wrongly apply the /admin rule there).
// "/" matches everything and acts as the catch-all.
type RouteRule struct {
	// PathPrefix must be absolute and canonical: "/", "/admin", "/agents/run".
	PathPrefix string
	// RequireAny lists realm roles; the caller needs at least one. Never empty.
	RequireAny []string
}

// Config holds proxy configuration.
type Config struct {
	// BackendURL is the upstream service, e.g. http://localhost:8000
	BackendURL string
	// JWKSUrl is the IdP's key endpoint, e.g. http://localhost:8090/realms/groot/...
	JWKSUrl string
	// AllowedIssuers and AllowedAudiences are passed to the token validator.
	AllowedIssuers   []string
	AllowedAudiences []string
	// Routes is the authorization policy. REQUIRED: New() refuses an empty
	// policy (fail closed). A path matching no rule is denied (default deny).
	Routes []RouteRule
	// MaxBodyBytes limits request body size. Default 10MB.
	MaxBodyBytes int64
	// SessionCookieName is the cookie carrying the access token. Default "groot_session".
	SessionCookieName string
	// Logger receives auth/authz denials with their real reasons (which are
	// deliberately NOT sent to the client). Default slog.Default().
	Logger *slog.Logger
}

// Proxy is the Groot IAM gateway reverse proxy.
// Implements http.Handler. Safe for concurrent use.
type Proxy struct {
	config    Config
	routes    []RouteRule // validated, sorted longest-prefix first; read-only after New
	validator *token.Validator
	rp        *httputil.ReverseProxy
	log       *slog.Logger
}

// New creates a Proxy from cfg.
// Returns an error if BackendURL is unparseable or the route policy is unsafe.
func New(cfg Config) (*Proxy, error) {
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 10 << 20 // 10 MB
	}
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = "groot_session"
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	routes, err := compileRoutes(cfg.Routes)
	if err != nil {
		return nil, err
	}

	backendURL, err := url.Parse(cfg.BackendURL)
	if err != nil {
		return nil, fmt.Errorf("gateway: parse backend URL %q: %w", cfg.BackendURL, err)
	}

	validator := token.NewValidator(cfg.JWKSUrl, cfg.AllowedIssuers, cfg.AllowedAudiences)

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Rewrite the request URL to point at the backend.
			// X-Verified-User and X-Verified-Roles are already set on req by
			// ServeHTTP before this Director runs — they travel to the backend.
			req.URL.Scheme = backendURL.Scheme
			req.URL.Host = backendURL.Host
			req.Host = backendURL.Host
			// Suppress User-Agent if not set, to avoid forwarding Go's default.
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "")
			}
		},
		// ErrorHandler is called when the backend is unreachable or the body
		// copy fails (e.g. MaxBytesReader limit exceeded).
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil && strings.Contains(err.Error(), "request body too large") {
				http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
		// FlushInterval for streaming responses (SSE used by agent_service chat).
		FlushInterval: 100 * time.Millisecond,
	}

	return &Proxy{
		config:    cfg,
		routes:    routes,
		validator: validator,
		rp:        rp,
		log:       logger,
	}, nil
}

// compileRoutes validates the policy and sorts it longest-prefix first.
//
// WHY so strict? Each rejected shape is a way to silently open access:
//   - empty policy          -> everything would be unmatched (fail closed instead)
//   - empty RequireAny      -> ambiguous: "nobody" or "anyone"? refuse to guess
//   - non-canonical prefix  -> "/admin/" or "/a/../admin" would never match
//     the canonical request paths we enforce, leaving the subtree unguarded
//   - duplicate prefix      -> which rule wins would depend on sort stability
func compileRoutes(in []RouteRule) ([]RouteRule, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("gateway: Config.Routes is empty; refusing to start without an authorization policy (fail closed)")
	}
	seen := make(map[string]bool, len(in))
	out := make([]RouteRule, 0, len(in))
	for i, r := range in {
		if !strings.HasPrefix(r.PathPrefix, "/") || path.Clean(r.PathPrefix) != r.PathPrefix {
			return nil, fmt.Errorf("gateway: route %d: PathPrefix %q must be absolute and canonical (no trailing slash, no . or ..)", i, r.PathPrefix)
		}
		if len(r.RequireAny) == 0 {
			return nil, fmt.Errorf("gateway: route %d (%s): RequireAny is empty; every route must name at least one role", i, r.PathPrefix)
		}
		if seen[r.PathPrefix] {
			return nil, fmt.Errorf("gateway: route %d: duplicate PathPrefix %q", i, r.PathPrefix)
		}
		seen[r.PathPrefix] = true
		// Copy so later mutation of the caller's slice can't change the policy.
		out = append(out, RouteRule{PathPrefix: r.PathPrefix, RequireAny: append([]string(nil), r.RequireAny...)})
	}
	sort.Slice(out, func(a, b int) bool { return len(out[a].PathPrefix) > len(out[b].PathPrefix) })
	return out, nil
}

// isCanonicalPath reports whether p is already in clean form.
//
// WHY reject instead of cleaning? Path-confusion bypass: "/agents/../admin/x"
// textually matches the /agents rule, but the backend may normalise it to
// "/admin/x". If the gateway authorises one path and the backend serves
// another, authorisation is meaningless. Refusing anything that is not
// already canonical removes the ambiguity. r.URL.Path is percent-DECODED, so
// "%2e%2e" and "%2F" tricks are caught too. A single trailing slash is allowed
// ("/agents/") because path.Clean strips it but it is not a traversal.
func isCanonicalPath(p string) bool {
	if p == "" || p[0] != '/' {
		return false
	}
	c := path.Clean(p)
	if p != "/" && strings.HasSuffix(p, "/") {
		c += "/"
	}
	return c == p
}

// matchRoute returns the longest RouteRule covering reqPath, or nil (default deny).
func (p *Proxy) matchRoute(reqPath string) *RouteRule {
	for i := range p.routes {
		pre := p.routes[i].PathPrefix
		if pre == "/" || reqPath == pre || strings.HasPrefix(reqPath, pre+"/") {
			return &p.routes[i]
		}
	}
	return nil
}

// NewServer returns an *http.Server wrapping the proxy with production timeouts.
//
// WHY explicit timeouts?
// Go's default http.Server has no timeouts. A slow or malicious client that
// opens a connection and never sends headers will hold that goroutine forever.
// ReadHeaderTimeout stops that in 5 seconds. WriteTimeout gives the backend
// 60 seconds — enough for slow LLM inference. IdleTimeout reclaims keep-alive
// connections that are unused.
func NewServer(addr string, p *Proxy) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           p,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB header limit
	}
}

// ServeHTTP implements http.Handler.
//
// Order is critical — do not reorder (see package doc).
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Security headers on response — always, even for 400/401/403/413
	for k, v := range securityResponseHeaders {
		w.Header().Set(k, v)
	}

	// 2. Strip identity headers — unconditional, regardless of auth outcome
	for _, h := range identityHeaders {
		r.Header.Del(h)
	}

	// 3. Canonical path check — before any routing or authorization decision.
	if !isCanonicalPath(r.URL.Path) {
		p.log.Warn("gateway: non-canonical path rejected", "path", r.URL.Path, "remote", r.RemoteAddr)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 4. Limit request body size — prevents backend OOM from huge uploads
	if r.ContentLength > p.config.MaxBodyBytes {
		http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, p.config.MaxBodyBytes)

	// 5a. Authenticate: extract token
	tokenStr := p.extractToken(r)
	if tokenStr == "" {
		w.Header().Set("WWW-Authenticate", wwwAuthNoToken)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 5b. Authenticate: validate (signature, exp, iss, aud, jti denylist).
	// The REAL reason goes to the server log only. Echoing it to the client
	// ("audience not in allow-list", "issuer mismatch") hands an attacker a
	// feedback loop for crafting tokens.
	claims, err := p.validator.Validate(tokenStr)
	if err != nil {
		p.log.Warn("gateway: token rejected", "reason", err.Error(), "path", r.URL.Path, "remote", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", wwwAuthInvalidToken)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 6. Authorize: longest matching rule; no rule = default deny.
	rule := p.matchRoute(r.URL.Path)
	allowed := false
	if rule != nil {
		for _, role := range rule.RequireAny {
			if claims.HasRole(role) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		required := "<no matching rule: default deny>"
		if rule != nil {
			required = strings.Join(rule.RequireAny, "|")
		}
		p.log.Warn("gateway: forbidden", "sub", claims.Subject, "path", r.URL.Path, "required", required, "remote", r.RemoteAddr)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// 7. Inject verified identity headers.
	// These replace whatever the client sent (already stripped in step 2).
	// The backend receives ONLY what the gateway injects here.
	r.Header.Set("X-Verified-User", claims.Subject)
	r.Header.Set("X-Verified-Roles", strings.Join(claims.Roles(), ","))

	// 8. Forward to backend
	p.rp.ServeHTTP(w, r)
}

// extractToken reads the Bearer token from Authorization header or session cookie.
func (p *Proxy) extractToken(r *http.Request) string {
	// Authorization: Bearer <token> takes precedence (API clients)
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// Session cookie (browser clients — set by OIDC callback in Phase 2)
	if cookie, err := r.Cookie(p.config.SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// Validator exposes the underlying token.Validator for tuning in tests.
func (p *Proxy) Validator() *token.Validator { return p.validator }
