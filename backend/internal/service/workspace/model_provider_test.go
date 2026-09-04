package workspace

import (
	"context"
	"errors"
	"net/http"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestModelProviderMutationsRequireAdministrator(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user-1"})
	tests := []struct {
		name string
		call func() error
	}{
		{name: "create connection", call: func() error {
			_, err := service.CreateModelProviderConnection(ctx, &workspacev1.CreateModelProviderConnectionRequest{})
			return err
		}},
		{name: "update connection", call: func() error {
			_, err := service.UpdateModelProviderConnection(ctx, &workspacev1.UpdateModelProviderConnectionRequest{})
			return err
		}},
		{name: "delete connection", call: func() error {
			_, err := service.DeleteModelProviderConnection(ctx, &workspacev1.DeleteModelProviderConnectionRequest{})
			return err
		}},
		{name: "refresh models", call: func() error {
			_, err := service.RefreshProviderModels(ctx, &workspacev1.RefreshProviderModelsRequest{})
			return err
		}},
		{name: "add model", call: func() error {
			_, err := service.CreateProviderModel(ctx, &workspacev1.CreateProviderModelRequest{})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := kratoserrors.Code(test.call()); code != http.StatusForbidden {
				t.Fatalf("ordinary User mutation code = %d, want %d", code, http.StatusForbidden)
			}
		})
	}
}

func TestApplyCatalogResultKeepsDefaultCatalogUnverified(t *testing.T) {
	connection := domain.ModelProviderConnection{ProviderType: "alibaba_bailian", VerificationStatus: "unverified"}
	models := []domain.ProviderModel{{ModelID: "qwen-plus"}}

	result := applyCatalogResult(&connection, workspaceapplication.ModelCatalogResult{Models: models, Source: "default"}, nil)

	if len(result) != 1 || connection.VerificationStatus != "unverified" || connection.VerificationError != "" {
		t.Fatalf("unexpected maintained catalog result: connection=%+v models=%+v", connection, result)
	}
}

func TestApplyCatalogResultRecordsDiscoveryFailureWithoutDefaults(t *testing.T) {
	connection := domain.ModelProviderConnection{ProviderType: "openai", VerificationStatus: "unverified"}

	result := applyCatalogResult(&connection, workspaceapplication.ModelCatalogResult{Models: []domain.ProviderModel{{ModelID: "stale"}}}, errors.New("provider unavailable"))

	if result != nil || connection.VerificationError != "provider unavailable" || connection.VerificationStatus != "unverified" {
		t.Fatalf("unexpected failed discovery result: connection=%+v models=%+v", connection, result)
	}
}
