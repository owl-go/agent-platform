package workspace

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"golang.org/x/crypto/bcrypt"
)

type workflowAPIContextKey struct{}
type workflowCredentialContextKey struct{}

type workflowCredentialContext struct {
	WorkflowID string
	OwnerID    string
	APIKey     string
	SecretHash string
}

type workflowTokenClaims struct {
	Audience   string `json:"aud"`
	WorkflowID string `json:"workflow_id"`
	OwnerID    string `json:"sub"`
	APIKey     string `json:"api_key"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

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
				ctx = context.WithValue(ctx, workflowCredentialContextKey{}, workflowCredentialContext{WorkflowID: workflowID, OwnerID: ownerID, APIKey: key, SecretHash: secretHash})
				next.ServeHTTP(writer, request.WithContext(ctx))
				return
			}
			if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
				writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
				return
			}
			workflowID, workflowRoute := workflowTokenRoute(request.Method, request.URL.Path)
			if workflowRoute {
				if access, ok := verifyWorkflowToken(request.Context(), workspace, strings.TrimSpace(token)); ok && access.WorkflowID == workflowID {
					ctx := accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: access.OwnerID})
					ctx = context.WithValue(ctx, workflowAPIContextKey{}, access.WorkflowID)
					next.ServeHTTP(writer, request.WithContext(ctx))
					return
				}
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
	if len(parts) == 5 && method == http.MethodPost && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workflows" && parts[4] == "api-token" {
		return parts[3], parts[3] != ""
	}
	return "", false
}

func workflowTokenRoute(method, path string) (string, bool) {
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

func issueWorkflowToken(access workflowCredentialContext, now time.Time) (string, time.Time, error) {
	expires := now.Add(15 * time.Minute)
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, err := json.Marshal(workflowTokenClaims{Audience: "agent-platform-workflow", WorkflowID: access.WorkflowID, OwnerID: access.OwnerID, APIKey: access.APIKey, IssuedAt: now.Unix(), ExpiresAt: expires.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	signature := hmac.New(sha256.New, []byte(access.SecretHash))
	_, _ = signature.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature.Sum(nil)), expires, nil
}

func verifyWorkflowToken(ctx context.Context, workspace *workspaceapplication.Service, token string) (workflowCredentialContext, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return workflowCredentialContext{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return workflowCredentialContext{}, false
	}
	var claims workflowTokenClaims
	if json.Unmarshal(payload, &claims) != nil || claims.Audience != "agent-platform-workflow" || claims.WorkflowID == "" || claims.APIKey == "" || claims.OwnerID == "" || time.Now().Unix() >= claims.ExpiresAt {
		return workflowCredentialContext{}, false
	}
	ownerID, secretHash, err := workspace.Repository().ResolveWorkflowCredential(ctx, claims.WorkflowID, claims.APIKey)
	if err != nil || ownerID != claims.OwnerID {
		return workflowCredentialContext{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return workflowCredentialContext{}, false
	}
	mac := hmac.New(sha256.New, []byte(secretHash))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return workflowCredentialContext{}, false
	}
	return workflowCredentialContext{WorkflowID: claims.WorkflowID, OwnerID: ownerID, APIKey: claims.APIKey, SecretHash: secretHash}, true
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
