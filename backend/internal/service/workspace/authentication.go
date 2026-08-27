package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"golang.org/x/crypto/bcrypt"
)

type workflowAPIContextKey struct{}

func NewAuthenticationFilter(accounts *accountapplication.Service, workspace *workspaceapplication.Service) (kratoshttp.FilterFunc, error) {
	if accounts == nil || workspace == nil {
		return nil, fmt.Errorf("Account and Agent Workspace services are required")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/healthz" || request.URL.Path == "/readyz" {
				next.ServeHTTP(writer, request)
				return
			}
			scheme, token, found := strings.Cut(request.Header.Get("Authorization"), " ")
			if found && strings.EqualFold(scheme, "Basic") {
				workflowID, allowed := workflowCredentialRoute(request.Method, request.URL.Path)
				key, secret, valid := request.BasicAuth()
				if !allowed || !valid || strings.TrimSpace(key) == "" || strings.TrimSpace(secret) == "" {
					writeAuthError(writer, http.StatusUnauthorized, "invalid_workflow_credential")
					return
				}
				ownerID, secretHash, err := workspace.Repository().ResolveWorkflowCredential(request.Context(), workflowID, key)
				if err != nil || bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(secret)) != nil {
					writeAuthError(writer, http.StatusUnauthorized, "invalid_workflow_credential")
					return
				}
				principal := accountdomain.Principal{UserID: ownerID}
				ctx := accountapplication.WithPrincipal(request.Context(), principal)
				ctx = context.WithValue(ctx, workflowAPIContextKey{}, workflowID)
				next.ServeHTTP(writer, request.WithContext(ctx))
				return
			}
			if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
				writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
				return
			}
			principal, err := accounts.Authenticate(request.Context(), strings.TrimSpace(token))
			switch {
			case err == nil:
				next.ServeHTTP(writer, request.WithContext(accountapplication.WithPrincipal(request.Context(), principal)))
			case errors.Is(err, accountdomain.ErrUnauthenticated):
				writeAuthError(writer, http.StatusUnauthorized, "invalid_authentication")
			default:
				writeAuthError(writer, http.StatusServiceUnavailable, "authentication_unavailable")
			}
		})
	}, nil
}

func workflowCredentialRoute(method, path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 5 && method == http.MethodPost && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workflows" && parts[4] == "runs" {
		return parts[3], parts[3] != ""
	}
	if len(parts) == 6 && method == http.MethodGet && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workflows" && parts[4] == "runs" {
		return parts[3], parts[3] != "" && parts[5] != ""
	}
	if len(parts) == 7 && method == http.MethodGet && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workflows" && parts[4] == "runs" && parts[6] == "events" {
		return parts[3], parts[3] != "" && parts[5] != ""
	}
	return "", false
}

func workflowAPIAccess(ctx context.Context, workflowID string) bool {
	allowed, ok := ctx.Value(workflowAPIContextKey{}).(string)
	return ok && allowed == workflowID
}

func writeAuthError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
