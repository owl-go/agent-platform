package gormrepo

import (
	"sync"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"gorm.io/gorm/schema"
)

func TestAssistantRecordPreservesSelectedProviderModel(t *testing.T) {
	row := assistantRecordFromDomain(domain.SmartAssistant{Name: "产品助手", ProviderModelID: "model-1"})
	if row.ProviderModelID != "model-1" || assistantFromRecord(row).ProviderModelID != "model-1" {
		t.Fatalf("selected Provider Model was not preserved: %+v", row)
	}
}

func TestAssistantSessionOwnerUsesDatabaseColumn(t *testing.T) {
	mapping, err := schema.Parse(&assistantSessionRecord{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	field := mapping.LookUpField("OwnerID")
	if field == nil {
		t.Fatal("OwnerID field was not mapped")
	}
	if field.DBName != "owner_user_id" {
		t.Fatalf("OwnerID database column = %q, want owner_user_id", field.DBName)
	}
}
