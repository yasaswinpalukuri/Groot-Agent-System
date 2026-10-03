// Package jwks fetches and caches JSON Web Key Sets from an OIDC provider.
//
// WHY a separate package?
// The JWKS cache has one job: given a key ID (kid), return the public key.
// Keeping it separate from token validation lets us test each piece in
// isolation and swap the cache implementation later.
package jwks

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksResponse is the JSON shape OIDC providers return at their certs endpoint.
// e.g. http://keycloak:8090/realms/groot/protocol/openid-connect/certs
type jwksResponse struct {
	Keys []jsonWebKey `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"` // key type: "RSA" or "EC"
	Kid string `json:"kid"` // key ID — matched against the JWT header kid
	Use string `json:"use"` // "sig" = signing key; we skip encryption keys
	Alg string `json:"alg"` // declared alg — informational only; we enforce our own allow-list
	// RSA fields
	N string `json:"n"` // modulus, base64url-encoded big integer
	E string `json:"e"` // public exponent, base64url-encoded
	// EC fields
	Crv string `json:"crv"` // curve name, e.g. "P-256"
	X   string `json:"x"`   // x coordinate, base64url-encoded
	Y   string `json:"y"`   // y coordinate, base64url-encoded
}

// Cache holds the most recently fetched public keys and enforces a minimum
// interval between refetches.
//
// WHY rate-limit refetches?
// An attacker can craft JWTs with arbitrary kid values. Without a floor on
// refetch frequency, every unknown kid forces an outbound HTTP call to
// Keycloak. With 1000 req/s, that becomes 1000 Keycloak calls per second —
// a trivial DoS. MinRefetch caps the blast at one call per interval
// regardless of request volume.
//
// Safe for concurrent use. The double-checked locking pattern (RLock first,
// then Lock only on miss) keeps the common path (cache hit) at read-lock
// cost only.
type Cache struct {
	url        string
	mu         sync.RWMutex
	keys       map[string]crypto.PublicKey
	lastFetch  time.Time
	MinRefetch time.Duration // floor between JWKS fetches; default 5 minutes
	HTTPClient *http.Client
}

// NewCache creates a Cache for the given JWKS URL.
// The first fetch is lazy — it happens on the first GetKey call.
func NewCache(url string) *Cache {
	return &Cache{
		url:        url,
		keys:       make(map[string]crypto.PublicKey),
		MinRefetch: 5 * time.Minute,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetKey returns the public key for kid, refetching JWKS if needed.
// Returns an error if the kid is unknown and a refetch is rate-limited.
func (c *Cache) GetKey(kid string) (crypto.PublicKey, error) {
	// Fast path: read lock only (no fetch needed)
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}

	// Slow path: acquire write lock, re-check, maybe refetch
	c.mu.Lock()
	defer c.mu.Unlock()

	// Another goroutine may have fetched while we waited for the write lock
	if key, ok = c.keys[kid]; ok {
		return key, nil
	}

	// Rate-limit check: skip if we fetched recently
	if !c.lastFetch.IsZero() && time.Since(c.lastFetch) < c.MinRefetch {
		remaining := c.MinRefetch - time.Since(c.lastFetch)
		return nil, fmt.Errorf("jwks: unknown kid %q; rate-limited, retry in %v", kid, remaining)
	}

	if err := c.fetch(); err != nil {
		return nil, fmt.Errorf("jwks: fetch failed: %w", err)
	}

	key, ok = c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("jwks: kid %q not found after JWKS refresh", kid)
	}
	return key, nil
}

// fetch does the HTTP GET and parses the JWKS response.
// Caller must hold c.mu (write lock).
func (c *Cache) fetch() error {
	resp, err := c.HTTPClient.Get(c.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	var result jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode JWKS JSON: %w", err)
	}

	newKeys := make(map[string]crypto.PublicKey, len(result.Keys))
	for _, k := range result.Keys {
		// Skip non-signing keys (e.g. encryption keys have use="enc")
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		var pub crypto.PublicKey
		switch k.Kty {
		case "RSA":
			pub, err = parseRSAPublicKey(k)
		case "EC":
			pub, err = parseECPublicKey(k)
		default:
			continue // silently skip unknown key types
		}
		if err != nil {
			return fmt.Errorf("parse jwk kid=%q: %w", k.Kid, err)
		}
		newKeys[k.Kid] = pub
	}

	c.keys = newKeys
	c.lastFetch = time.Now()
	return nil
}

// parseRSAPublicKey decodes the base64url-encoded n and e fields.
//
// WHY do this manually instead of a library?
// It is 10 lines of stdlib. Knowing how JWK RSA keys are encoded (big-endian
// big integers, base64url without padding) is an interview staple.
func parseRSAPublicKey(k jsonWebKey) (*rsa.PublicKey, error) {
	if k.N == "" || k.E == "" {
		return nil, errors.New("RSA key missing n or e fields")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent e: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	return &rsa.PublicKey{N: n, E: e}, nil
}

// parseECPublicKey decodes the base64url-encoded x and y coordinates.
func parseECPublicKey(k jsonWebKey) (*ecdsa.PublicKey, error) {
	if k.X == "" || k.Y == "" {
		return nil, errors.New("EC key missing x or y fields")
	}
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}
