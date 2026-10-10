package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
)

func TestTeamCommandApprovalsReflectAllActiveMembersAndCloseOnCancellation(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	stages, _ := json.Marshal([]domain.ExpertStage{{InvocationID: ids[0], Position: 1, State: "running"}, {InvocationID: ids[1], Position: 2, State: "running"}, {InvocationID: ids[2], Position: 3, State: "queued"}})
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Updates(map[string]any{"response_snapshot": []byte(`{"schema_version":3}`), "expert_stages": stages}).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 2)
	wait := func(index int) {
		go func() {
			digest := strings.Repeat("a", 64)
			_, err := fixture.repository.Await(ctx, cliconnector.ApprovalRequest{OwnerID: fixture.ownerID, ExecutionKind: "session", ExecutionID: fmt.Sprint(fixture.assistant.AssistantMessageID), StageID: ids[index], ConnectorName: "Tool", Operation: "write", Target: "Safe target", CommandDigest: digest, Nonce: "nonce-" + ids[index], Identity: cliconnector.IdentityUser, AllowedIdentities: []cliconnector.Identity{cliconnector.IdentityUser}, CommandDigests: map[cliconnector.Identity]string{cliconnector.IdentityUser: digest}, ExpiresAt: time.Now().Add(time.Minute)})
			result <- err
		}()
	}
	awaitCount := func(want int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			approvals, err := fixture.repository.ListCommandApprovals(ctx, fixture.ownerID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(approvals) == want {
				return
			}
			select {
			case err := <-result:
				t.Fatalf("approval stopped: %v", err)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("approval did not become visible")
	}
	assertState := func(want string) {
		t.Helper()
		var state string
		fixture.db.Model(&messageRecord{}).Select("state").Where("id = ?", fixture.assistant.AssistantMessageID).Scan(&state)
		if state != want {
			t.Fatalf("state=%s want=%s", state, want)
		}
	}
	wait(0)
	awaitCount(1)
	assertState("generating")
	wait(1)
	awaitCount(2)
	assertState("waiting_for_user")
	// A queued task is visible but does not count as an executing member.
	if err := fixture.repository.RecordProgress(ctx, fixture.assistant, application.ExecutionEvent{Type: "expert.stage.updated", Payload: []byte(fmt.Sprintf(`{"invocation_id":%q,"position":3,"state":"queued"}`, ids[2]))}); err != nil {
		t.Fatal(err)
	}
	assertState("waiting_for_user")
	cancel()
	for i := 0; i < 2; i++ {
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	assertState("generating")
	var pending int64
	fixture.db.Table("cli_command_approvals").Where("owner_user_id = ? AND state IN ('pending','approved')", fixture.ownerID).Count(&pending)
	if pending != 0 {
		t.Fatal("cancelled approvals remained consumable")
	}
}
