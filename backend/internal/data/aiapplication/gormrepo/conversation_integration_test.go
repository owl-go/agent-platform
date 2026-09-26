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
	conversation, err := repository.CreateAssistantConversation(ctx, domain.AssistantConversation{OwnerID: owner, AssistantID: assistantID, AssistantName: "测试助手", Welcome: "你好", AssistantSnapshot: domain.SmartAssistant{ID: assistantID, Name: "测试助手"}, ModelSnapshot: domain.AssistantModel{ModelID: "test-model"}})
	if err != nil {
		t.Fatal(err)
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
	completed, err := repository.FinishAssistantTurn(ctx, owner, conversation.ID, turn.ID, "completed", "model", "", "partial", 3, 4)
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
}
