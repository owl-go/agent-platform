package workspace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/skillstore"
	"agent-platform/backend/internal/workspacefs"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	workspacev1.UnimplementedAgentWorkspaceServiceServer
	accounts  *accountapplication.Service
	credits   *creditsapplication.Service
	workspace *workspaceapplication.Service
	box       *secretcrypto.Box
	files     *workspacefs.Store
	skills    *skillstore.Store
	objects   objectstore.Provider
	config    platformconfig.Config
}

func (service *Service) RegisterHTTP(server *kratoshttp.Server) {
	workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, service)
	server.Handle("/api/v1/sessions/{session_id}/messages/{message_id}/events", http.HandlerFunc(service.streamSessionMessage))
	server.Handle("/api/v1/workflows/{workflow_id}/runs/{run_id}/events", http.HandlerFunc(service.streamRunEvents))
	server.Handle("/api/v1/workflows/{workflow_id}/workspace/download", http.HandlerFunc(service.downloadWorkspaceFile))
	server.Handle("/api/v1/attachments/upload", http.HandlerFunc(service.uploadAttachment))
	server.Handle("/api/v1/attachments/{attachment_id}/download", http.HandlerFunc(service.downloadAttachment))
}

func New(accounts *accountapplication.Service, credits *creditsapplication.Service, workspace *workspaceapplication.Service, box *secretcrypto.Box, files *workspacefs.Store, skills *skillstore.Store, objects objectstore.Provider, config platformconfig.Config) (*Service, error) {
	if accounts == nil || credits == nil || workspace == nil || box == nil || files == nil || skills == nil || objects == nil {
		return nil, fmt.Errorf("Account, Credits, Agent Workspace, encryption, Workspace File, Skill, and Object Store services are required")
	}
	return &Service{accounts: accounts, credits: credits, workspace: workspace, box: box, files: files, skills: skills, objects: objects, config: config}, nil
}

func (service *Service) owner(ctx context.Context) (string, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return "", publicError(err)
	}
	return principal.UserID, nil
}

func (service *Service) validateExecutionRuntimes(ctx context.Context, owner string, expertID, teamID *string) error {
	check := func(expert workspacedomain.Expert) error {
		availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{expert})
		if err != nil {
			return err
		}
		if !availability[expert.ID].Available {
			return fmt.Errorf("%w: selected Expert execution profile is unavailable", workspacedomain.ErrInvalid)
		}
		return nil
	}
	if expertID != nil {
		expert, err := service.workspace.Repository().GetExpert(ctx, owner, *expertID)
		if err != nil {
			return err
		}
		return check(expert)
	}
	if teamID != nil {
		team, err := service.workspace.Repository().GetExpertTeam(ctx, owner, *teamID)
		if err != nil {
			return err
		}
		for _, expert := range team.Experts {
			if err := check(expert); err != nil {
				return err
			}
		}
		return nil
	}
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return err
	}
	runtime, exists := service.config.Worker.Runtimes[string(settings.DefaultRuntimeEngine)]
	if !exists || !runtime.Available {
		return fmt.Errorf("%w: default Runtime Engine is unavailable", workspacedomain.ErrInvalid)
	}
	return nil
}

func (service *Service) validateExpertInputAvailability(ctx context.Context, input workspacedomain.ExpertInput) error {
	expert := workspacedomain.Expert{ID: "candidate", ExecutionInstruction: input.ExecutionInstruction, ProviderModelID: input.ProviderModelID, RuntimeEngine: input.RuntimeEngine}
	availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{expert})
	if err != nil {
		return err
	}
	if !availability[expert.ID].Available {
		return fmt.Errorf("%w: Expert execution profile is unavailable", workspacedomain.ErrInvalid)
	}
	return nil
}

func publicError(err error) error {
	switch {
	case errors.Is(err, accountdomain.ErrUnauthenticated):
		return kratoserrors.New(http.StatusUnauthorized, "authentication_required", "authentication required")
	case errors.Is(err, accountdomain.ErrForbidden):
		return kratoserrors.New(http.StatusForbidden, "access_denied", "access denied")
	case errors.Is(err, accountdomain.ErrNotFound), errors.Is(err, workspacedomain.ErrNotFound):
		return kratoserrors.New(http.StatusNotFound, "resource_not_found", "resource not found")
	case errors.Is(err, accountdomain.ErrConflict), errors.Is(err, workspacedomain.ErrConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "version_conflict", "resource version changed")
	case errors.Is(err, workspacedomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, creditsdomain.ErrInsufficientCredits):
		return kratoserrors.New(http.StatusTooManyRequests, "insufficient_credits", "Credit Balance is not positive")
	case errors.Is(err, creditsdomain.ErrCodeUnavailable):
		return kratoserrors.New(http.StatusUnprocessableEntity, "redemption_code_unavailable", "Redemption Code is unavailable")
	case errors.Is(err, creditsdomain.ErrConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "credit_conflict", "Credits state changed")
	case errors.Is(err, creditsdomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	default:
		return kratoserrors.New(http.StatusInternalServerError, "request_failed", "request failed")
	}
}

func randomCredential(prefix string, size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(hash), err
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
