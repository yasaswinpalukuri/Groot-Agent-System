// Package gateway implements the Groot IAM reverse proxy.
//
// The proxy sits in front of every Groot service. On each request it:
//  1. Strips client-supplied identity headers (unconditional — before any auth)
//  2. Validates the JWT (Phase 1 validator via JWKS)
//  3. Injects verified identity headers so backends never do their own auth
//  4. Forwards to the backend via httputil.ReverseProxy
//  5. Adds security headers to every response
//
// WHY strip before validate, not after?
// If we validated first and only stripped on failure, a race condition could
// let a crafted request slip through with injected headers during the
// validation window. Stripping unconditionally — before any other logic —
// means the backend can never see a client-supplied identity header, period.
//
// WHY httputil.ReverseProxy?
// It handles connection pooling, keep-alive, hop-by-hop header removal
// (Connection, Upgrade, etc.), and X-Forwarded-For automatically. Writing
// this from scratch with http.Client is tedious and error-prone.
package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	"X-Verified-User",     // our own injected header — strip before re-injecting
	"X-Verified-Roles",    // our own injected roles header
	"X-User",              // used by nginx auth_request and many frameworks
	"X-Remote-User",       // Apache mod_auth convention
	"X-Auth-User",         // common in internal proxies
	"X-Forwarded-User",    // used by some SSO systems (distinct from X-Forwarded-For)
	"X-Role",              // singular role header
	"X-Roles",             // plural role header
}

// securityResponseHeaders are added to every response — including 401s.
// Adding them only on success would mean error pages have weaker security.
var securityResponseHeaders = map[string]string{
	// HSTS: tell browsers to always use HTTPS for 1 year, including subdomains.
	// WHY? Prevents protocol downgrade attacks (SSLstrip).
	// Note: only effective when TLS is active (Phase 4). Harmless on HTTP dev.
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

// Config holds proxy configuration.
type Config struct {
	// BackendURL is the upstream service, e.g. http://localhost:8000
	BackendURL string
	// JWKSUrl is the IdP's key endpoint, e.g. http://localhost:8090/realms/groot/...
	JWKSUrl string
	// AllowedIssuers and AllowedAudiences are passed to the token validator.
	AllowedIssuers   []string
	AllowedAudiences []string
	// MaxBodyBytes limits request body size. Default 10MB.
	MaxBodyBytes int64
	// SessionCookieName is the cookie carrying the access token. Default "groot_session".
	SessionCookieName string
}

// Proxy is the Groot IAM gateway reverse proxy.
// Implements http.Handler. Safe for concurrent use.
type Proxy struct {
	config    Config
	validator *token.Validator
	rp        *httputil.ReverseProxy
}

// New creates a Proxy from cfg.
// Returns an error if BackendURL is unparseable.
func New(cfg Config) (*Proxy, error) {
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 10 << 20 // 10 MB
	}
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = "groot_session"
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
		validator: validator,
		rp:        rp,
	}, nil
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
// Order is critical — do not reorder:
//  1. Add security headers to response (always present, even on 401)
//  2. Strip identity headers from request (unconditional)
//  3. Enforce body size limit
//  4. Extract token (Authorization: Bearer or session cookie)
//  5. Validate token
//  6. Inject verified identity headers
//  7. Forward to backend
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Security headers on response — always, even for 401/413 responses
	for k, v := range securityResponseHeaders {
		w.Header().Set(k, v)
	}

	// 2. Strip identity headers — unconditional, regardless of auth outcome
	// This is the critical step: even if we return 401 below, the client
	// cannot trick us into forwarding these headers.
	for _, h := range identityHeaders {
		r.Header.Del(h)
	}

	// 3. Limit request body size — prevents backend OOM from huge uploads
	if r.ContentLength > p.config.MaxBodyBytes {
		http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, p.config.MaxBodyBytes)

	// 4. Extract token
	tokenStr := p.extractToken(r)
	if tokenStr == "" {
		http.Error(w, "unauthorized: no token", http.StatusUnauthorized)
		return
	}

	// 5. Validate token (signature, exp, iss, aud, jti denylist)
	claims, err := p.validator.Validate(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 6. Inject verified identity headers.
	// These replace whatever the client sent (already stripped in step 2).
	// The backend receives ONLY what the gateway injects here.
	r.Header.Set("X-Verified-User", claims.Subject)
	r.Header.Set("X-Verified-Roles", strings.Join(claims.Roles(), ","))

	// 7. Forward to backend
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
