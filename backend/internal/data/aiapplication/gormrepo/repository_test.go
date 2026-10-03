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

func TestAssistantRecordPreservesEmbeddingAndIndependentWidgetIcon(t *testing.T) {
	assistant := domain.SmartAssistant{ID: "assistant", OwnerID: "owner", Name: "助手", Icon: "sparkles", Share: domain.ShareConfiguration{EmbedType: "floating", WidgetDefaultOpen: true, WidgetIcon: "ai-applications/assistant-icons/owner/2a748f68-95a5-41df-8a53-488e335c223c", Width: "400px", Height: 600}}
	stored := assistantFromRecord(assistantRecordFromDomain(assistant))
	if stored.Share.EmbedType != "floating" || !stored.Share.WidgetDefaultOpen || stored.Share.WidgetIcon != assistant.Share.WidgetIcon || stored.Icon != "sparkles" {
		t.Fatalf("embedding changed in storage: %#v", stored)
	}
}
