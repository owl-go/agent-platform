package workspace

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestApplyDiscoveryResultKeepsMaintainedCatalogUnverified(t *testing.T) {
	connection := domain.ModelProviderConnection{ProviderType: "alibaba_bailian", VerificationStatus: "unverified"}
	models := []domain.ProviderModel{{ModelID: "qwen-plus"}}

	result := applyDiscoveryResult(&connection, models, nil)

	if len(result) != 1 || connection.VerificationStatus != "unverified" || connection.VerificationError != "" {
		t.Fatalf("unexpected maintained catalog result: connection=%+v models=%+v", connection, result)
	}
}

func TestApplyDiscoveryResultRecordsDynamicDiscoveryFailure(t *testing.T) {
	connection := domain.ModelProviderConnection{ProviderType: "openai", VerificationStatus: "unverified"}

	result := applyDiscoveryResult(&connection, []domain.ProviderModel{{ModelID: "stale"}}, errors.New("provider unavailable"))

	if result != nil || connection.VerificationError != "provider unavailable" || connection.VerificationStatus != "unverified" {
		t.Fatalf("unexpected failed discovery result: connection=%+v models=%+v", connection, result)
	}
}
