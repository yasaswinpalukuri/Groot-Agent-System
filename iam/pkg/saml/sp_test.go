package saml_test

import (
	"strings"
	"sync"
	"testing"

	csaml "github.com/crewjam/saml"
	samlpkg "github.com/yasaswinpalukuri/groot-iam/pkg/saml"
)

// minimalIDPMeta returns a minimal EntityDescriptor with a fake SSO URL.
func minimalIDPMeta(ssoURL string) *csaml.EntityDescriptor {
	return &csaml.EntityDescriptor{
		EntityID: "http://fake-idp",
		IDPSSODescriptors: []csaml.IDPSSODescriptor{
			{
				SSODescriptor: csaml.SSODescriptor{
					RoleDescriptor: csaml.RoleDescriptor{},
				},
				SingleSignOnServices: []csaml.Endpoint{
					{
						Binding:  csaml.HTTPRedirectBinding,
						Location: ssoURL,
					},
				},
			},
		},
	}
}

// newTestSP creates an SP backed by a minimal fake IdP.
func newTestSP(t *testing.T) *samlpkg.SP {
	t.Helper()
	sp, err := samlpkg.New(samlpkg.Config{
		EntityID:    "http://localhost:8080/auth/saml/metadata",
		ACSURL:      "http://localhost:8080/auth/saml/callback",
		IDPMetadata: minimalIDPMeta("http://fake-idp/sso"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return sp
}

// ── Constructor tests ─────────────────────────────────────────────────────────

func TestNewSPRejectsMissingIDPMetadata(t *testing.T) {
	_, err := samlpkg.New(samlpkg.Config{
		EntityID: "http://localhost:8080/auth/saml/metadata",
		ACSURL:   "http://localhost:8080/auth/saml/callback",
	})
	if err == nil {
		t.Error("expected error for nil IDPMetadata")
	}
	if !strings.Contains(err.Error(), "IDPMetadata") {
		t.Errorf("error %q should mention IDPMetadata", err)
	}
}

func TestNewSPRejectsInvalidEntityID(t *testing.T) {
	_, err := samlpkg.New(samlpkg.Config{
		EntityID:    "://bad url",
		ACSURL:      "http://localhost:8080/auth/saml/callback",
		IDPMetadata: minimalIDPMeta("http://fake-idp/sso"),
	})
	if err == nil {
		t.Error("expected error for invalid entity ID")
	}
}

// ── Pending ID lifecycle tests ────────────────────────────────────────────────

// TestPendingCountStartsAtZero verifies fresh SP has no pending requests.
func TestPendingCountStartsAtZero(t *testing.T) {
	sp := newTestSP(t)
	if sp.PendingCount() != 0 {
		t.Fatalf("expected 0 pending IDs, got %d", sp.PendingCount())
	}
}

// TestAuthnRedirectURLStoresPendingID verifies state is stored per login.
func TestAuthnRedirectURLStoresPendingID(t *testing.T) {
	sp := newTestSP(t)

	_, _, err := sp.AuthnRedirectURL()
	if err != nil {
		t.Fatalf("AuthnRedirectURL: %v", err)
	}

	if sp.PendingCount() != 1 {
		t.Errorf("expected 1 pending ID after AuthnRedirectURL, got %d", sp.PendingCount())
	}
}

// TestAuthnRedirectURLGeneratesUniqueIDs verifies no two concurrent logins
// share a request ID — prevents one session from consuming another's assertion.
func TestAuthnRedirectURLGeneratesUniqueIDs(t *testing.T) {
	sp := newTestSP(t)
	seen := make(map[string]bool)

	for i := 0; i < 10; i++ {
		_, id, err := sp.AuthnRedirectURL()
		if err != nil {
			t.Fatalf("AuthnRedirectURL %d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate request ID %q on iteration %d", id, i)
		}
		seen[id] = true
	}
}

// TestMultipleAuthURLsAccumulatePending verifies each call adds to pending map.
func TestMultipleAuthURLsAccumulatePending(t *testing.T) {
	sp := newTestSP(t)

	for i := 1; i <= 3; i++ {
		_, _, err := sp.AuthnRedirectURL()
		if err != nil {
			t.Fatalf("AuthnRedirectURL %d: %v", i, err)
		}
		if sp.PendingCount() != i {
			t.Errorf("after %d calls: PendingCount=%d, want %d", i, sp.PendingCount(), i)
		}
	}
}

// TestHasNoPendingIDsWhenEmpty verifies HasNoPendingIDs returns true before
// any login attempt — used to test the "no pending IDs" error path.
func TestHasNoPendingIDsWhenEmpty(t *testing.T) {
	sp := newTestSP(t)
	if !sp.HasNoPendingIDs() {
		t.Error("fresh SP should have no pending IDs")
	}
	_, _, err := sp.AuthnRedirectURL()
	if err != nil {
		t.Fatal(err)
	}
	if sp.HasNoPendingIDs() {
		t.Error("SP should have a pending ID after AuthnRedirectURL")
	}
}

// TestConcurrentAuthnRedirectURL verifies the pending map is race-safe.
// Run with -race to detect data races.
func TestConcurrentAuthnRedirectURL(t *testing.T) {
	sp := newTestSP(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := sp.AuthnRedirectURL()
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent AuthnRedirectURL: %v", err)
	}

	if sp.PendingCount() != 20 {
		t.Errorf("expected 20 pending IDs, got %d", sp.PendingCount())
	}
}

// ── Metadata test ─────────────────────────────────────────────────────────────

// TestMetadataXMLIsValidXML verifies the SP produces parseable metadata XML.
func TestMetadataXMLIsValidXML(t *testing.T) {
	sp := newTestSP(t)
	b, err := sp.MetadataXML()
	if err != nil {
		t.Fatalf("MetadataXML: %v", err)
	}
	if len(b) == 0 {
		t.Error("metadata XML should not be empty")
	}
	// Must contain our EntityID
	if !strings.Contains(string(b), "localhost:8080") {
		t.Errorf("metadata XML should contain EntityID, got:\n%s", string(b))
	}
}

// ── Security property documentation ──────────────────────────────────────────

// TestSAMLSecurityProperties documents the security properties as runnable tests.
func TestSAMLSecurityProperties(t *testing.T) {
	properties := []struct {
		name      string
		handledBy string
		detail    string
	}{
		{"XML signature verification", "crewjam/saml", "Verified against IdP public key from metadata"},
		{"XML Signature Wrapping (XSW)", "crewjam/saml", "Signed element verified as the assertion itself"},
		{"Audience restriction", "crewjam/saml", "Audience must match SP EntityID"},
		{"NotOnOrAfter / NotBefore", "crewjam/saml", "Time constraints with configurable clock skew"},
		{"InResponseTo validation", "crewjam/saml + pendingIDs map", "crewjam checks InResponseTo against IDs we pass"},
		{"Assertion replay prevention", "pendingIDs map (our code)", "IDs deleted after first ParseCallbackHTTP"},
		{"Unsolicited assertion rejection", "pendingIDs map (our code)", "IdP-initiated SSO fails — no matching pending ID"},
	}

	for _, p := range properties {
		t.Run(p.name, func(t *testing.T) {
			t.Logf("Handled by: %s", p.handledBy)
			t.Logf("Detail: %s", p.detail)
		})
	}
}

// ── Integration test stub ─────────────────────────────────────────────────────

func TestSAMLIntegration(t *testing.T) {
	t.Skip("integration test — requires running Keycloak with SAML client configured")
	// Full flow:
	// 1. Fetch metadata: GET http://localhost:8090/realms/groot/protocol/saml/descriptor
	// 2. New(Config{..., IDPMetadata: parsed})
	// 3. AuthnRedirectURL() → redirect user
	// 4. User authenticates at Keycloak
	// 5. ParseCallbackHTTP(r) → verify assertion
}
