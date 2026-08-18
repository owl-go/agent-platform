package tokenverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	"agent-platform/backend/internal/platformconfig"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
)

type OIDCVerifier struct {
	verifier          *coreoidc.IDTokenVerifier
	organizationClaim string
}

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

	discoveryClient := &http.Client{
		Transport: unavailableTransport{base: transport},
		Timeout:   config.DiscoveryTimeout.Value(),
	}
	discoveryContext, cancel := context.WithTimeout(ctx, config.DiscoveryTimeout.Value())
	defer cancel()
	discoveryContext = coreoidc.ClientContext(discoveryContext, discoveryClient)
	provider, err := coreoidc.NewProvider(discoveryContext, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC Provider: %w", err)
	}
	var metadata struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil || strings.TrimSpace(metadata.JWKSURI) == "" {
		return nil, fmt.Errorf("OIDC Provider discovery did not return a valid jwks_uri")
	}

	jwksClient := &http.Client{
		Transport: unavailableTransport{base: transport},
		Timeout:   config.JWKSTimeout.Value(),
	}
	jwksContext := coreoidc.ClientContext(context.Background(), jwksClient)
	keys := availabilityKeySet{inner: coreoidc.NewRemoteKeySet(jwksContext, metadata.JWKSURI)}
	verifier := coreoidc.NewVerifier(config.Issuer, keys, &coreoidc.Config{
		ClientID:             config.Audience,
		SupportedSigningAlgs: append([]string(nil), config.SigningAlgorithms...),
	})
	return &OIDCVerifier{verifier: verifier, organizationClaim: config.OrganizationClaim}, nil
}

func (verifier *OIDCVerifier) Verify(ctx context.Context, rawToken string) (identitydomain.VerifiedIdentity, error) {
	if verifier == nil || verifier.verifier == nil || strings.TrimSpace(rawToken) == "" {
		return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
	}
	state := &verificationState{}
	verified, err := verifier.verifier.Verify(context.WithValue(ctx, verificationStateKey{}, state), rawToken)
	if err != nil {
		if state.unavailable != nil {
			return identitydomain.VerifiedIdentity{}, fmt.Errorf("OIDC JWKS unavailable: %w", state.unavailable)
		}
		return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
	}
	var claims map[string]json.RawMessage
	if err := verified.Claims(&claims); err != nil {
		return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
	}
	var organizationSlug string
	if err := json.Unmarshal(claims[verifier.organizationClaim], &organizationSlug); err != nil {
		return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
	}
	identity := identitydomain.VerifiedIdentity{
		Subject:          strings.TrimSpace(verified.Subject),
		OrganizationSlug: strings.TrimSpace(organizationSlug),
	}
	if identity.Subject == "" || identity.OrganizationSlug == "" {
		return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
	}
	return identity, nil
}

type verificationStateKey struct{}

type verificationState struct {
	unavailable error
}

type availabilityKeySet struct {
	inner coreoidc.KeySet
}

func (keySet availabilityKeySet) VerifySignature(ctx context.Context, rawToken string) ([]byte, error) {
	payload, err := keySet.inner.VerifySignature(ctx, rawToken)
	if err == nil {
		return payload, nil
	}
	var unavailable *upstreamUnavailableError
	if errors.As(err, &unavailable) {
		if state, ok := ctx.Value(verificationStateKey{}).(*verificationState); ok {
			state.unavailable = unavailable
		}
	}
	return nil, err
}

type unavailableTransport struct {
	base http.RoundTripper
}

func (transport unavailableTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return nil, &upstreamUnavailableError{cause: err}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = response.Body.Close()
		return nil, &upstreamUnavailableError{status: response.StatusCode}
	}
	return response, nil
}

type upstreamUnavailableError struct {
	status int
	cause  error
}

func (err *upstreamUnavailableError) Error() string {
	if err.status != 0 {
		return fmt.Sprintf("OIDC upstream returned HTTP %d", err.status)
	}
	return "OIDC upstream request failed"
}

func (err *upstreamUnavailableError) Unwrap() error { return err.cause }

var _ interface {
	Verify(context.Context, string) (identitydomain.VerifiedIdentity, error)
} = (*OIDCVerifier)(nil)

var _ http.RoundTripper = unavailableTransport{}
