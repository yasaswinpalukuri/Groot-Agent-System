// Package oidc implements OIDC Authorization Code + PKCE and OAuth 2.0
// Client Credentials flows for the Groot IAM gateway.
//
// Design decisions:
//
// 1. State is stored server-side and consumed on first use.
//    This prevents CSRF (state mismatch) and callback replay (single-use).
//
// 2. PKCE S256 is mandatory (never Plain).
//    Plain provides no protection because the code_challenge IS the verifier.
//
// 3. Nonce is verified after code exchange.
//    This prevents an attacker from replaying a valid ID token from one
//    session into a different session.
//
// 4. Cookies are HttpOnly + SameSite=Strict.
//    HttpOnly: JavaScript cannot read the token (defeats XSS token theft).
//    SameSite=Strict: cookie not sent on cross-site requests (defeats CSRF).
//    Secure is set to false in development; Phase 4 enables it with TLS.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	// SessionCookieName is the cookie that carries the access token.
	SessionCookieName = "groot_session"
	// stateExpiry is how long a pending login state is valid.
	// 10 minutes is enough for a human to complete the login flow.
	stateExpiry = 10 * time.Minute
	// SessionExpiry is the max-age of the session cookie.
	SessionExpiry = 8 * time.Hour
)

// Config holds OIDC/OAuth2 configuration.
type Config struct {
	IssuerURL    string   // e.g. http://localhost:8090/realms/groot
	ClientID     string   // e.g. groot-gateway
	ClientSecret string   // from Keycloak client settings
	RedirectURL  string   // e.g. http://localhost:8080/auth/callback
	Scopes       []string // defaults to openid, profile, email
}

// stateEntry holds the per-login PKCE verifier and nonce.
// It is stored server-side to prevent tampering.
type stateEntry struct {
	nonce        string
	codeVerifier string
	expiry       time.Time
}

// Handler implements OIDC Authorization Code + PKCE and helper methods
// for the Groot gateway. Safe for concurrent use.
type Handler struct {
	config   Config
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config

	mu     sync.Mutex
	states map[string]stateEntry // state → entry (single-use)
}

// New discovers the OIDC provider at cfg.IssuerURL and returns a Handler.
// Keycloak must be running when this is called — it fetches the discovery doc.
func New(ctx context.Context, cfg Config) (*Handler, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover %q: %w", cfg.IssuerURL, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	return &Handler{
		config:   cfg,
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
		states: make(map[string]stateEntry),
	}, nil
}

// AuthURL registers a login attempt and returns the URL to redirect the
// user's browser to.
//
// state: a random value you generated with GenerateState()
// nonce: a random value you generated with GenerateNonce()
//
// Both are stored server-side. The state ties the callback to this login
// attempt. The nonce ties the ID token to this login attempt.
func (h *Handler) AuthURL(state, nonce string) (string, error) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}

	h.mu.Lock()
	h.states[state] = stateEntry{
		nonce:        nonce,
		codeVerifier: verifier,
		expiry:       time.Now().Add(stateExpiry),
	}
	h.mu.Unlock()

	return h.oauth2.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", CodeChallenge(verifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

// StateCount returns the number of pending state entries.
// Exported for testing only — not part of the public API.
func (h *Handler) StateCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.states)
}

// Callback processes the IdP's redirect back to our callback URL.
// Returns the verified ID token and OAuth2 token on success.
//
// Validation order:
//  1. state present in query → CSRF check
//  2. state known and not expired → replay + timeout check
//  3. state consumed (delete on read) → prevents callback URL replay
//  4. code present → completeness check
//  5. code exchanged with code_verifier → PKCE integrity
//  6. id_token extracted and signature verified → cryptographic integrity
//  7. nonce matches stored value → prevents ID token replay across sessions
func (h *Handler) Callback(ctx context.Context, r *http.Request) (*oidc.IDToken, *oauth2.Token, error) {
	state := r.URL.Query().Get("state")
	if state == "" {
		return nil, nil, errors.New("oidc: missing state parameter")
	}

	// Look up AND immediately delete the state entry.
	// Deleting before any network call prevents two concurrent callbacks
	// from both succeeding with the same state (race condition replay).
	h.mu.Lock()
	entry, ok := h.states[state]
	delete(h.states, state)
	h.mu.Unlock()

	if !ok {
		return nil, nil, fmt.Errorf("oidc: unknown or already-used state %q", state)
	}
	if time.Now().After(entry.expiry) {
		return nil, nil, errors.New("oidc: login state expired — start login again")
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		return nil, nil, fmt.Errorf("oidc: IdP error %q: %s",
			errParam, r.URL.Query().Get("error_description"))
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, nil, errors.New("oidc: missing code parameter")
	}

	// Exchange the authorization code + code_verifier for tokens.
	// The code_verifier proves we are the same party that started the login.
	oauth2Token, err := h.oauth2.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", entry.codeVerifier),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: code exchange: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, nil, errors.New("oidc: token response missing id_token")
	}

	// Verify the ID token signature and standard claims (exp, iss, aud)
	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: ID token verify: %w", err)
	}

	// Nonce check — the nonce in the ID token must match what we sent.
	// This prevents an attacker from replaying a valid ID token from one
	// session into a different session's callback.
	var nonceClaims struct {
		Nonce string `json:"nonce"`
	}
	if err := idToken.Claims(&nonceClaims); err != nil {
		return nil, nil, fmt.Errorf("oidc: extract nonce claim: %w", err)
	}
	if nonceClaims.Nonce != entry.nonce {
		return nil, nil, errors.New("oidc: nonce mismatch — possible ID token replay attack")
	}

	return idToken, oauth2Token, nil
}

// SetSessionCookie writes the access token to a cookie.
// secure: set true when TLS is enabled (Phase 4+).
func SetSessionCookie(w http.ResponseWriter, accessToken string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(SessionExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie expires the session cookie, logging the user out.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClientCredentialsToken fetches an access token using the Client Credentials
// grant. Use this for service-to-service auth (n8n → gateway, etc.).
//
// WHY not share a password?
// The token is short-lived (typically 5 minutes). Even if it leaks, the blast
// radius is bounded. The client secret rotates independently of application
// configuration. The gateway validates the token via JWKS — no shared secret
// between services.
func ClientCredentialsToken(ctx context.Context, tokenURL, clientID, clientSecret string) (*oauth2.Token, error) {
	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: tokenURL},
	}
	token, err := cfg.Exchange(ctx, "",
		oauth2.SetAuthURLParam("grant_type", "client_credentials"),
	)
	if err != nil {
		return nil, fmt.Errorf("client_credentials %q: %w", clientID, err)
	}
	return token, nil
}

// ── PKCE helpers ─────────────────────────────────────────────────────────────

// GenerateCodeVerifier returns a cryptographically random base64url string.
// Length 43 chars (32 bytes encoded) — within the RFC 7636 §4.1 range of 43–128.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CodeChallenge computes BASE64URL(SHA256(verifier)) per RFC 7636 §4.6.
// This is the value sent in the authorization request.
// The verifier is sent in the token request to prove ownership.
func CodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState returns a cryptographically random 32-hex-char string.
func GenerateState() (string, error) { return randomHex(16) }

// GenerateNonce returns a cryptographically random 32-hex-char string.
func GenerateNonce() (string, error) { return randomHex(16) }

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
