package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	identityapplication "agent-platform/backend/internal/biz/identity/application"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func NewAuthenticationFilter(access *identityapplication.AccessService) (kratoshttp.FilterFunc, error) {
	if access == nil {
		return nil, fmt.Errorf("Identity Access Service is required")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/healthz" || request.URL.Path == "/readyz" {
				next.ServeHTTP(writer, request)
				return
			}
			token, ok := bearerToken(request)
			if !ok {
				writeAuthenticationError(writer, http.StatusUnauthorized, "authentication_required")
				return
			}
			principal, err := access.Authenticate(request.Context(), token)
			switch {
			case err == nil:
				ctx := identityapplication.WithPrincipal(request.Context(), principal)
				next.ServeHTTP(writer, request.WithContext(ctx))
			case errors.Is(err, identitydomain.ErrUnauthenticated):
				writeAuthenticationError(writer, http.StatusUnauthorized, "invalid_authentication")
			default:
				writeAuthenticationError(writer, http.StatusServiceUnavailable, "authentication_unavailable")
			}
		})
	}, nil
}

func bearerToken(request *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(request.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func writeAuthenticationError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
