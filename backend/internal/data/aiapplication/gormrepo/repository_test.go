package gormrepo

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

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
