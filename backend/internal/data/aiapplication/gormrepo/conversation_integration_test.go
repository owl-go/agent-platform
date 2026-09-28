package gormrepo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
)

func TestAssistantConversationAuditAndSingleActiveTurn(t *testing.T) {
	db := rateLimitTestDatabase(t)
	ctx := context.Background()
	owner := uuid.NewString()
	other := uuid.NewString()
	for _, id := range []string{owner, other} {
		if err := db.Exec("INSERT INTO users (id, oidc_subject, username, email, display_name) VALUES (?, ?, ?, ?, ?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := New(db, nil)
	assistantID := uuid.NewString()
	conversation, err := repository.CreateAssistantConversation(ctx, domain.AssistantConversation{OwnerID: owner, AssistantID: assistantID, AssistantName: "测试助手", Welcome: "你好", AssistantSnapshot: domain.SmartAssistant{ID: assistantID, Name: "测试助手", ProviderModelID: "model-1"}, ModelSnapshot: domain.AssistantModel{ProviderModelID: "model-1", ModelID: "test-model", Protocol: "openai_chat"}})
	if err != nil {
		t.Fatal(err)
	}
	if frozen, err := repository.GetAssistantConversation(ctx, owner, conversation.ID); err != nil || frozen.ModelSnapshot.ProviderModelID != "model-1" || frozen.ModelSnapshot.Protocol != "openai_chat" {
		t.Fatalf("frozen model = %+v, err = %v", frozen.ModelSnapshot, err)
	}
	if _, err := repository.GetAssistantConversation(ctx, other, conversation.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner read: %v", err)
	}
	var group sync.WaitGroup
	start := make(chan struct{})
	results := make(chan domain.AssistantTurn, 2)
	errorsFound := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			turn, beginErr := repository.BeginAssistantTurn(ctx, owner, conversation.ID, "问题")
			if beginErr != nil {
				errorsFound <- beginErr
			} else {
				results <- turn
			}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errorsFound)
	if len(results) != 1 || len(errorsFound) != 1 || !errors.Is(<-errorsFound, domain.ErrConflict) {
		t.Fatalf("concurrent turns: accepted=%d rejected=%d", len(results), len(errorsFound))
	}
	turn := <-results
	if err := repository.SaveAssistantTurnProgress(ctx, other, conversation.ID, turn.ID, "secret"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner write: %v", err)
	}
	if err := repository.SaveAssistantTurnProgress(ctx, owner, conversation.ID, turn.ID, "partial"); err != nil {
		t.Fatal(err)
	}
	if err := repository.CancelAssistantTurn(ctx, owner, conversation.ID, turn.ID); err != nil {
		t.Fatal(err)
	}
	if turns, err := repository.ListAssistantTurns(ctx, owner, conversation.ID); err != nil || turns[0].State != "cancelled" {
		t.Fatalf("cancel was not durable before provider exit: %+v, %v", turns, err)
	}
	if _, err := repository.BeginAssistantTurn(ctx, owner, conversation.ID, "第二问"); err != nil {
		t.Fatal(err)
	}
	completed, err := repository.FinishAssistantTurn(ctx, owner, conversation.ID, turn.ID, "completed", "model", "", "partial", "", 3, 4)
	if err != nil || completed.State != "cancelled" {
		t.Fatalf("cancelled finish: %+v, %v", completed, err)
	}
	history, err := repository.ListAssistantTurns(ctx, owner, conversation.ID)
	if err != nil || len(history) != 2 || history[0].Answer != "partial" {
		t.Fatalf("audit history: %+v, %v", history, err)
	}
	if err := db.Exec("UPDATE assistant_conversation_turns SET updated_at = ? WHERE id = ?", time.Now().Add(-11*time.Minute), history[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.BeginAssistantTurn(ctx, owner, conversation.ID, "第三问"); err != nil {
		t.Fatal(err)
	}
	history, err = repository.ListAssistantTurns(ctx, owner, conversation.ID)
	if err != nil || len(history) != 3 || history[1].State != "failed" || history[1].Error != "interrupted" {
		t.Fatalf("stale recovery: %+v, %v", history, err)
	}
	if err := repository.MarkAssistantInterruptedCreditsReleased(ctx, other, conversation.ID, history[1].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner marks credits: %v", err)
	}
	if err := repository.MarkAssistantInterruptedCreditsReleased(ctx, owner, conversation.ID, history[1].ID); err != nil {
		t.Fatal(err)
	}
	history, err = repository.ListAssistantTurns(ctx, owner, conversation.ID)
	if err != nil || history[1].Error != "credits_released" {
		t.Fatalf("released credit marker: %+v, %v", history, err)
	}
	failed, err := repository.FinishAssistantTurn(ctx, owner, conversation.ID, history[2].ID, "failed", "", "", "", "model_authentication", 0, 0)
	if err != nil || failed.Error != "model_authentication" {
		t.Fatalf("safe failure code was not persisted: %+v, %v", failed, err)
	}
	history, err = repository.ListAssistantTurns(ctx, owner, conversation.ID)
	if err != nil || history[2].Error != "model_authentication" {
		t.Fatalf("safe failure code was not restored: %+v, %v", history, err)
	}
}

func TestAssistantProviderModelPersistsAcrossEdits(t *testing.T) {
	db := rateLimitTestDatabase(t)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users (id, oidc_subject, username, email, display_name) VALUES (?, ?, ?, ?, ?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(db, nil)
	created, err := repository.CreateAssistant(ctx, owner, domain.SmartAssistant{Name: "模型助手", ProviderModelID: "model-1", State: domain.StateDraft})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.GetAssistant(ctx, owner, created.ID)
	if err != nil || loaded.ProviderModelID != "model-1" {
		t.Fatalf("created model = %q, err = %v", loaded.ProviderModelID, err)
	}
	loaded.ProviderModelID = "model-2"
	updated, err := repository.UpdateAssistant(ctx, owner, created.ID, loaded, loaded.Version)
	if err != nil || updated.ProviderModelID != "model-2" {
		t.Fatalf("updated model = %q, err = %v", updated.ProviderModelID, err)
	}
}

func TestPublicAssistantConversationIsVisitorScopedAndHiddenFromPrivateHistory(t *testing.T) {
	db := rateLimitTestDatabase(t)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users (id, oidc_subject, username, email, display_name) VALUES (?, ?, ?, ?, ?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(db, nil)
	assistantID := uuid.NewString()
	created, err := repository.CreateAssistantConversation(ctx, domain.AssistantConversation{OwnerID: owner, AssistantID: assistantID, VisitorHash: "visitor-a", ShareTokenRevision: 3, AssistantName: "共享助手", AssistantSnapshot: domain.SmartAssistant{ID: assistantID, Name: "共享助手"}, ModelSnapshot: domain.AssistantModel{ProviderModelID: "model-1", Protocol: "openai_chat"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetAssistantConversation(ctx, owner, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private read of public conversation: %v", err)
	}
	if history, err := repository.ListAssistantConversations(ctx, owner, assistantID); err != nil || len(history) != 0 {
		t.Fatalf("private history = %+v, err = %v", history, err)
	}
	for _, scope := range []struct {
		visitor  string
		revision int64
	}{{"visitor-b", 3}, {"visitor-a", 4}, {"", 3}, {"visitor-a", 0}} {
		if _, err := repository.GetPublicAssistantConversation(ctx, owner, assistantID, created.ID, scope.visitor, scope.revision); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-scope read: %v", err)
		}
	}
	loaded, err := repository.GetPublicAssistantConversation(ctx, owner, assistantID, created.ID, "visitor-a", 3)
	if err != nil || loaded.ModelSnapshot.ProviderModelID != "model-1" || loaded.ModelSnapshot.Protocol != "openai_chat" {
		t.Fatalf("public snapshot = %+v, err = %v", loaded.ModelSnapshot, err)
	}
	turn, err := repository.BeginAssistantTurn(ctx, owner, created.ID, "问题")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FinishAssistantTurn(ctx, owner, created.ID, turn.ID, "completed", "model", "", "答案", "", 2, 3); err != nil {
		t.Fatal(err)
	}
	turns, err := repository.ListAssistantTurns(ctx, owner, created.ID)
	if err != nil || len(turns) != 1 || turns[0].Answer != "答案" {
		t.Fatalf("public audit turns = %+v, err = %v", turns, err)
	}
}
