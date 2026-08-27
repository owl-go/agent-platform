package tokenverifier

import (
	"context"
	"fmt"
	"net/http"
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
	client := &http.Client{Transport: transport, Timeout: config.DiscoveryTimeout.Value()}
	discoveryCtx, cancel := context.WithTimeout(coreoidc.ClientContext(ctx, client), config.DiscoveryTimeout.Value())
	defer cancel()
	provider, err := coreoidc.NewProvider(discoveryCtx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC Provider: %w", err)
	}
	return &OIDCVerifier{verifier: provider.Verifier(&coreoidc.Config{ClientID: config.Audience, SupportedSigningAlgs: append([]string(nil), config.SigningAlgorithms...)})}, nil
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
