package tokenverifier

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/platformconfig"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
)

type OIDCVerifier struct{ verifier *coreoidc.IDTokenVerifier }

func NewOIDC(ctx context.Context, config platformconfig.AuthenticationConfig, transport http.RoundTripper) (*OIDCVerifier, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.Mode != "oidc" {
		return nil, fmt.Errorf("OIDC verifier requires authentication.mode oidc")
	}
	if transport == nil {
		transport = http.DefaultTransport
	}
	if config.InternalBaseURL != "" {
		issuer, _ := url.Parse(config.Issuer)
		internal, _ := url.Parse(config.InternalBaseURL)
		transport = loopbackTransport{base: transport, issuer: issuer, internal: internal}
	}
	client := &http.Client{Transport: transport, Timeout: config.DiscoveryTimeout.Value()}
	discoveryCtx, cancel := context.WithTimeout(coreoidc.ClientContext(ctx, client), config.DiscoveryTimeout.Value())
	defer cancel()
	provider, err := coreoidc.NewProvider(discoveryCtx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC Provider: %w", err)
	}
	return &OIDCVerifier{verifier: provider.Verifier(&coreoidc.Config{ClientID: config.Audience, SupportedSigningAlgs: append([]string(nil), config.SigningAlgorithms...)})}, nil
}

// Only this trusted issuer's discovery and key paths use the co-located
// transport. Provider discovery and JWT verification still enforce its public
// issuer, audience, signature and expiry. No TLS verification is disabled.
type loopbackTransport struct {
	base             http.RoundTripper
	issuer, internal *url.URL
}

func (transport loopbackTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != transport.issuer.Scheme || request.URL.Host != transport.issuer.Host || !strings.HasPrefix(request.URL.Path, strings.TrimSuffix(transport.issuer.Path, "/")+"/") {
		return transport.base.RoundTrip(request)
	}
	clone := request.Clone(request.Context())
	value := *request.URL
	value.Scheme, value.Host = transport.internal.Scheme, transport.internal.Host
	clone.URL = &value
	clone.Host = transport.issuer.Host
	return transport.base.RoundTrip(clone)
}

func (verifier *OIDCVerifier) Verify(ctx context.Context, rawToken string) (accountdomain.VerifiedIdentity, error) {
	if verifier == nil || verifier.verifier == nil || strings.TrimSpace(rawToken) == "" {
		return accountdomain.VerifiedIdentity{}, accountdomain.ErrUnauthenticated
	}
	token, err := verifier.verifier.Verify(ctx, rawToken)
	if err != nil || strings.TrimSpace(token.Subject) == "" {
		return accountdomain.VerifiedIdentity{}, accountdomain.ErrUnauthenticated
	}
	return accountdomain.VerifiedIdentity{Subject: strings.TrimSpace(token.Subject)}, nil
}
