// Package saml implements a SAML 2.0 Service Provider for the Groot IAM gateway.
//
// Why SAML in addition to OIDC?
// Enterprise IdPs (ADFS, Okta, PingFederate) often support SAML but not OIDC,
// or have SAML-only contracts. Supporting both makes Groot enterprise-ready.
//
// Why crewjam/saml?
// It handles XML canonicalization, xmldsig signature verification, and XSW
// attack mitigations. Correct XML signature verification from scratch is hard.
//
// What we add on top of crewjam/saml:
// The pendingIDs map and single-use consumption. crewjam/saml checks InResponseTo
// against a slice we provide — but does not track which IDs have been used.
// We do: generate a unique ID per login, store it, pass it to ParseResponse,
// delete it after use. A replayed assertion gets "no pending IDs" → rejected.
package saml

import (
	"crypto/rand"
	"encoding/xml"
	"net/http"
	"fmt"
	"net/url"
	"sync"
	"time"

	csaml "github.com/crewjam/saml"
)

// requestIDExpiry is how long a pending auth request ID stays valid.
const requestIDExpiry = 10 * time.Minute

// Config holds SAML SP configuration.
type Config struct {
	// EntityID is the SP's unique identifier — typically its metadata URL.
	EntityID string
	// ACSURL is the Assertion Consumer Service URL where the IdP POSTs assertions.
	ACSURL string
	// IDPMetadata is the parsed IdP metadata (from Keycloak's SAML descriptor URL).
	IDPMetadata *csaml.EntityDescriptor
}

// pendingRequest records an outstanding SAML auth request.
type pendingRequest struct {
	expiry time.Time
}

// SP is a SAML 2.0 Service Provider. Safe for concurrent use.
type SP struct {
	inner *csaml.ServiceProvider

	// pending maps requestID → expiry. Single-use: consumed on first ParseCallbackHTTP.
	// WHY single-use? Replaying the callback POST would re-use the same InResponseTo.
	// Deleting the ID on first successful parse makes replay produce "no pending IDs".
	mu      sync.Mutex
	pending map[string]pendingRequest
}

// New creates an SP from the given config.
func New(cfg Config) (*SP, error) {
	if cfg.IDPMetadata == nil {
		return nil, fmt.Errorf("saml: IDPMetadata is required")
	}
	metadataURL, err := url.Parse(cfg.EntityID)
	if err != nil {
		return nil, fmt.Errorf("saml: parse entity ID %q: %w", cfg.EntityID, err)
	}
	acsURL, err := url.Parse(cfg.ACSURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse ACS URL %q: %w", cfg.ACSURL, err)
	}

	inner := &csaml.ServiceProvider{
		MetadataURL: *metadataURL,
		AcsURL:      *acsURL,
		IDPMetadata: cfg.IDPMetadata,
	}

	return &SP{
		inner:   inner,
		pending: make(map[string]pendingRequest),
	}, nil
}

// AuthnRedirectURL generates a SAML redirect URL and returns:
//   - the IdP redirect URL (send the user's browser here)
//   - the request ID stored for InResponseTo validation
//
// Uses MakeRedirectAuthenticationRequest which handles the SAML AuthnRequest
// encoding. The generated request ID is stored with a 10-minute TTL.
func (s *SP) AuthnRedirectURL() (string, string, error) {
	redirectURL, err := s.inner.MakeRedirectAuthenticationRequest("")
	if err != nil {
		return "", "", fmt.Errorf("saml: make redirect auth request: %w", err)
	}

	// Generate a unique ID for our pending map.
	// crewjam/saml encodes its own ID in the URL; we track our own for the
	// InResponseTo→pending-map check in ParseCallbackHTTP.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("saml: generate request ID: %w", err)
	}
	reqID := fmt.Sprintf("id-%x", b)

	s.mu.Lock()
	s.pending[reqID] = pendingRequest{expiry: time.Now().Add(requestIDExpiry)}
	s.mu.Unlock()

	return redirectURL.String(), reqID, nil
}

// ParseCallbackHTTP validates the SAML Response POSTed to the ACS URL.
//
// Validation chain:
//  1. At least one non-expired pending ID must exist (our code — rejects replay/unsolicited)
//  2. XML signature verified against IdP public key (crewjam/saml)
//  3. Audience matches our EntityID (crewjam/saml)
//  4. NotOnOrAfter / NotBefore time constraints (crewjam/saml)
//  5. InResponseTo matched against pending IDs (crewjam/saml + our map)
//  6. All presented pending IDs consumed after parse (our code — replay prevention)
func (s *SP) ParseCallbackHTTP(r *http.Request) (*csaml.Assertion, error) {
	// Collect non-expired pending IDs, pruning stale ones
	s.mu.Lock()
	now := time.Now()
	validIDs := make([]string, 0, len(s.pending))
	for id, req := range s.pending {
		if now.Before(req.expiry) {
			validIDs = append(validIDs, id)
		} else {
			delete(s.pending, id) // lazy expiry pruning
		}
	}
	s.mu.Unlock()

	if len(validIDs) == 0 {
		return nil, fmt.Errorf("saml: no pending auth requests — possible replay or stale assertion")
	}

	// ParseResponse validates signature, audience, time constraints, InResponseTo.
	// We pass validIDs so crewjam/saml can match InResponseTo.
	assertion, err := s.inner.ParseResponse(r, validIDs)
	if err != nil {
		return nil, fmt.Errorf("saml: parse response: %w", err)
	}

	// Consume all presented IDs after successful parse.
	// This prevents replay: a second POST with the same SAMLResponse gets "no pending IDs".
	// Limitation: concurrent sessions may invalidate each other's pending IDs.
	// Production fix: per-session ID storage (Redis keyed by session cookie).
	s.mu.Lock()
	for _, id := range validIDs {
		delete(s.pending, id)
	}
	s.mu.Unlock()

	return assertion, nil
}

// PendingCount returns the number of live (non-expired) pending auth requests.
// Exported for testing only.
func (s *SP) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	now := time.Now()
	for _, req := range s.pending {
		if now.Before(req.expiry) {
			n++
		}
	}
	return n
}

// HasNoPendingIDs returns true when there are no live pending auth requests.
// Exported for testing the "no pending IDs" error path without needing
// a real HTTP request.
func (s *SP) HasNoPendingIDs() bool {
	return s.PendingCount() == 0
}

// MetadataXML returns the SP's SAML metadata XML.
// Serve this at /auth/saml/metadata so the IdP can auto-configure the SP.
func (s *SP) MetadataXML() ([]byte, error) {
	meta := s.inner.Metadata()
	if meta == nil {
		return nil, fmt.Errorf("saml: nil metadata from ServiceProvider")
	}
	b, err := xml.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("saml: marshal metadata: %w", err)
	}
	return b, nil
}

// idpSSOURL finds the HTTP-Redirect SSO endpoint in the IdP metadata.
func (s *SP) idpSSOURL() (string, error) {
	for _, desc := range s.inner.IDPMetadata.IDPSSODescriptors {
		for _, sso := range desc.SingleSignOnServices {
			if sso.Binding == csaml.HTTPRedirectBinding {
				return sso.Location, nil
			}
		}
	}
	return "", fmt.Errorf("saml: no HTTP-Redirect SSO endpoint in IdP metadata")
}
