package tokenverifier

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/platformconfig"
)

func TestColocatedDiscoveryStillChecksThePublicIssuer(t *testing.T) {
	issuer := "https://workspace.example:8443/identity/realms/agent-platform"
	reportedIssuer := issuer
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Host != "workspace.example:8443" || request.URL.Path != "/identity/realms/agent-platform/.well-known/openid-configuration" {
			t.Errorf("unexpected discovery destination: %s %s", request.Host, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"issuer":%q,"jwks_uri":%q}`, reportedIssuer, issuer+"/protocol/openid-connect/certs")
	}))
	defer server.Close()
	config := platformconfig.AuthenticationConfig{
		Mode: "oidc", Issuer: issuer, InternalBaseURL: server.URL, Audience: "agent-platform-api", ClientID: "agent-platform-web",
		RedirectURI: "https://workspace.example:8443/auth/callback", LogoutRedirectURI: "https://workspace.example:8443",
		SigningAlgorithms: []string{"RS256"}, DiscoveryTimeout: platformconfig.Duration(time.Second), JWKSTimeout: platformconfig.Duration(time.Second),
	}
	if _, err := NewOIDC(t.Context(), config, nil); err != nil {
		t.Fatal(err)
	}
	reportedIssuer = "https://attacker.example/identity/realms/agent-platform"
	if _, err := NewOIDC(t.Context(), config, nil); err == nil {
		t.Fatal("co-located transport accepted a different issuer")
	}
}

type recordingTransport struct{ destination string }

func (transport *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.destination = request.URL.String()
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

func TestLoopbackTransportDoesNotRewriteOtherOriginsOrRealmPaths(t *testing.T) {
	base := &recordingTransport{}
	issuerRequest, _ := http.NewRequestWithContext(context.Background(), "GET", "https://workspace.example/identity/realms/platform", nil)
	internalRequest, _ := http.NewRequestWithContext(context.Background(), "GET", "http://127.0.0.1:8081", nil)
	transport := loopbackTransport{base: base, issuer: issuerRequest.URL, internal: internalRequest.URL}
	for _, address := range []string{"https://other.example/identity/realms/platform/certs", "https://workspace.example/identity/realms/other/certs", "https://workspace.example/identity/realms/platform-other/certs"} {
		request, _ := http.NewRequestWithContext(t.Context(), "GET", address, nil)
		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if base.destination != address {
			t.Fatalf("unrelated destination was rewritten: %s", base.destination)
		}
	}
}
