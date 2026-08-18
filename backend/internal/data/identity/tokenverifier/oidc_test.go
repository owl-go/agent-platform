package tokenverifier

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	"agent-platform/backend/internal/platformconfig"
)

func TestOIDCVerifierAcceptsOnlyValidConfiguredIdentity(t *testing.T) {
	provider := newTestOIDCProvider(t)
	verifier, err := NewOIDC(context.Background(), provider.config(), provider.server.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}

	validClaims := map[string]any{
		"iss": provider.server.URL, "sub": "user-subject", "aud": "agent-platform-api",
		"exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "organization": "acme",
	}
	identity, err := verifier.Verify(context.Background(), provider.sign(t, provider.key, "current", validClaims))
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "user-subject" || identity.OrganizationSlug != "acme" {
		t.Fatalf("VerifiedIdentity = %+v", identity)
	}
	badSignature := provider.sign(t, provider.key, "current", validClaims)
	parts := strings.Split(badSignature, ".")
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	signature[0] ^= 0xff
	badSignature = parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(signature)
	if _, err := verifier.Verify(context.Background(), badSignature); !errors.Is(err, identitydomain.ErrUnauthenticated) {
		t.Fatalf("invalid signature error = %v, want ErrUnauthenticated", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "issuer", mutate: func(claims map[string]any) { claims["iss"] = "https://other.example.test" }},
		{name: "audience", mutate: func(claims map[string]any) { claims["aud"] = "other-api" }},
		{name: "expiry", mutate: func(claims map[string]any) { claims["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "not before", mutate: func(claims map[string]any) { claims["nbf"] = time.Now().Add(time.Hour).Unix() }},
		{name: "subject", mutate: func(claims map[string]any) { claims["sub"] = "" }},
		{name: "organization", mutate: func(claims map[string]any) { delete(claims, "organization") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := cloneClaims(validClaims)
			test.mutate(claims)
			_, err := verifier.Verify(context.Background(), provider.sign(t, provider.key, "current", claims))
			if !errors.Is(err, identitydomain.ErrUnauthenticated) {
				t.Fatalf("Verify() error = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

func TestOIDCVerifierDistinguishesUnknownKeyFromUnavailableJWKS(t *testing.T) {
	provider := newTestOIDCProvider(t)
	verifier, err := NewOIDC(context.Background(), provider.config(), provider.server.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{
		"iss": provider.server.URL, "sub": "user-subject", "aud": "agent-platform-api",
		"exp": time.Now().Add(time.Hour).Unix(), "organization": "acme",
	}
	unknown, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), provider.sign(t, unknown, "unknown", claims)); !errors.Is(err, identitydomain.ErrUnauthenticated) {
		t.Fatalf("unknown key error = %v, want ErrUnauthenticated", err)
	}

	provider.jwksUnavailable = true
	if _, err := verifier.Verify(context.Background(), provider.sign(t, unknown, "unavailable", claims)); err == nil || errors.Is(err, identitydomain.ErrUnauthenticated) {
		t.Fatalf("unavailable JWKS error = %v, want infrastructure error", err)
	}
}

func TestOIDCVerifierRejectsUnsafeJWKSURI(t *testing.T) {
	for _, jwksURI := range []string{
		"http://metadata.internal/keys",
		"https://keys.example.test/keys#fragment",
		"https://user@keys.example.test/keys",
		"/relative/keys",
	} {
		t.Run(jwksURI, func(t *testing.T) {
			provider := newTestOIDCProvider(t)
			provider.jwksURI = jwksURI
			if _, err := NewOIDC(context.Background(), provider.config(), provider.server.Client().Transport); err == nil {
				t.Fatal("NewOIDC accepted an unsafe jwks_uri")
			}
		})
	}
}

func TestOIDCVerifierAllowsHTTPSJWKSURIQuery(t *testing.T) {
	provider := newTestOIDCProvider(t)
	provider.jwksURI = provider.server.URL + "/keys?tenant=acme"
	if _, err := NewOIDC(context.Background(), provider.config(), provider.server.Client().Transport); err != nil {
		t.Fatalf("NewOIDC rejected a standards-compliant jwks_uri query: %v", err)
	}
}

type testOIDCProvider struct {
	server          *httptest.Server
	key             *rsa.PrivateKey
	jwksUnavailable bool
	jwksURI         string
}

func newTestOIDCProvider(t *testing.T) *testOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	provider := &testOIDCProvider{key: key}
	mux := http.NewServeMux()
	provider.server = httptest.NewTLSServer(mux)
	t.Cleanup(provider.server.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(writer http.ResponseWriter, _ *http.Request) {
		jwksURI := provider.jwksURI
		if jwksURI == "" {
			jwksURI = provider.server.URL + "/keys"
		}
		writeJSON(t, writer, map[string]any{
			"issuer": provider.server.URL, "jwks_uri": jwksURI,
			"authorization_endpoint": provider.server.URL + "/authorize", "token_endpoint": provider.server.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(writer http.ResponseWriter, _ *http.Request) {
		if provider.jwksUnavailable {
			http.Error(writer, "offline", http.StatusServiceUnavailable)
			return
		}
		writeJSON(t, writer, map[string]any{"keys": []any{rsaJWK(&provider.key.PublicKey, "current")}})
	})
	return provider
}

func (provider *testOIDCProvider) config() platformconfig.AuthenticationConfig {
	return platformconfig.AuthenticationConfig{
		Mode: "oidc", Issuer: provider.server.URL, Audience: "agent-platform-api", ClientID: "agent-platform-web",
		OrganizationClaim: "organization", RedirectURI: "https://app.example.test/auth/callback", LogoutRedirectURI: "https://app.example.test",
		SigningAlgorithms: []string{"RS256"}, DiscoveryTimeout: platformconfig.Duration(2 * time.Second), JWKSTimeout: platformconfig.Duration(2 * time.Second),
	}
}

func (provider *testOIDCProvider) sign(t *testing.T, key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": keyID, "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(encoded))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func rsaJWK(key *rsa.PublicKey, keyID string) map[string]any {
	exponent := big.NewInt(int64(key.E)).Bytes()
	return map[string]any{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID,
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(exponent),
	}
}

func cloneClaims(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
}
