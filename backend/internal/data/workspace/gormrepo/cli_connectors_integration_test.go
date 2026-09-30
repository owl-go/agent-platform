package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestCLIConnectorInstallAndReinstallWithDerivedAuthentication(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	input := cliconnector.Definition{Name: "Feishu CLI", Icon: "terminal", Description: "Read Feishu documents", InstallationType: "npm", Package: "@larksuite/cli", Version: "1.0.93"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatalf("install simplified CLI Definition: %v", err)
	}
	if item.AuthenticationDriver != "" {
		t.Fatal("draft must leave authentication unresolved for the package builder")
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET authentication_driver='feishu', state='disabled' WHERE id=?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, item.VersionNumber)
	if err != nil {
		t.Fatalf("reinstall simplified CLI Definition: %v", err)
	}
	if updated.State != cliconnector.StateDraft || updated.AuthenticationDriver != "" {
		t.Fatalf("reinstall did not reset derived metadata: %#v", updated)
	}
	if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, updated.VersionNumber); err != nil {
		t.Fatalf("queue CLI installation: %v", err)
	}
	if err := db.Exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID).Error; err == nil {
		t.Fatal("unresolved authentication must never become available")
	}
}

func TestCLIConnectorReinstallCanRecordRepeatedConformance(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	input := cliconnector.Definition{Name: "Example CLI", Icon: "terminal", Description: "Read examples", InstallationType: "npm", Package: "example-cli", Version: "1.0.0"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	result := cliconnector.BuildResult{State: cliconnector.StateAvailable, BundleObjectKey: "cli-connectors/bundles/test.zip", BundleSHA256: strings.Repeat("a", 64), RuntimeDigests: []string{"sha256:" + strings.Repeat("b", 64)}, Package: input.Package, Version: input.Version, Integrity: "sha512-test", Executable: "example", AuthenticationDriver: "none", SupportedArchitectures: []string{"linux-amd64"}, Capabilities: []cliconnector.Capability{{ID: "read", ArgvPrefix: []string{"read"}, Risk: cliconnector.RiskLow, Identities: []cliconnector.Identity{cliconnector.IdentityUser}, EgressHosts: []string{"example.test"}, Timeout: time.Minute}}}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, item.VersionNumber); err != nil {
			t.Fatal(err)
		}
		var job *application.ExecutionJob
		if err := db.Transaction(func(tx *gorm.DB) error { var err error; job, err = claimCLIConnectorBuild(tx); return err }); err != nil {
			t.Fatal(err)
		}
		if job == nil {
			t.Fatal("build was not queued")
		}
		if err := repository.FinishCLIConnectorBuild(ctx, *job, result, ""); err != nil {
			t.Fatalf("installation %d could not finish: %v", attempt+1, err)
		}
		available, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		verified, err := repository.HasCLIConnectorRuntimeConformance(ctx, item.ID, result.BundleSHA256, result.RuntimeDigests[0])
		if err != nil || !verified {
			t.Fatalf("exact bundle Runtime conformance unavailable: verified=%v err=%v", verified, err)
		}
		if attempt == 0 {
			item, err = repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, available.VersionNumber)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestBeginCLIConnectorAuthorizationReusesPendingAttempt(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner, definition, enablement := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO cli_connector_definitions(id,name,description,installation_type,npm_package,npm_version,npm_integrity,executable,state,authentication_driver,created_by_user_id) VALUES(?,?,?,'npm','@larksuite/cli','1.0.93','sha512-test','lark','available','feishu',?)", definition, "Feishu CLI", "Test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO cli_connector_enablements(id,owner_user_id,definition_id,state) VALUES(?,?,?,'enabled')", enablement, owner, definition).Error; err != nil {
		t.Fatal(err)
	}

	first, err := repository.BeginCLIConnectorAuthorization(ctx, owner, enablement, cliconnector.IdentityUser, []string{"im:chat:read"}, "https://accounts.feishu.cn/first", time.Now().Add(5*time.Minute), []byte("first-device"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.BeginCLIConnectorAuthorization(ctx, owner, enablement, cliconnector.IdentityUser, []string{"im:message"}, "https://accounts.feishu.cn/second", time.Now().Add(10*time.Minute), []byte("second-device"))
	if err != nil {
		t.Fatalf("retry pending authorization: %v", err)
	}
	if second.ID != first.ID || second.ActionURL != "https://accounts.feishu.cn/second" || len(second.Scopes) != 1 || second.Scopes[0] != "im:message" {
		t.Fatalf("reused authorization attempt = %#v, first = %#v", second, first)
	}
}

func TestDisableCLIConnectorPreservesApplicationAndAuthorization(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	owner, definition, enablement := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner)
	exec("INSERT INTO cli_connector_definitions(id,name,description,installation_type,npm_package,npm_version,npm_integrity,executable,state,authentication_driver,created_by_user_id) VALUES(?,?,?,'npm','@larksuite/cli','1.0.93','sha512-test','lark','available','feishu',?)", definition, "Feishu CLI", "Test", owner)
	exec("INSERT INTO cli_connector_enablements(id,owner_user_id,definition_id,state,version) VALUES(?,?,?,'enabled',3)", enablement, owner, definition)
	exec("INSERT INTO feishu_cli_applications(owner_user_id,enablement_id,provider_application_id_ciphertext,provider_application_secret_ciphertext,provider_name,developer_console_url) VALUES(?,?,'app','secret','Test','https://example.test')", owner, enablement)
	exec("INSERT INTO cli_connector_authorizations(owner_user_id,enablement_id,identity,external_identity_id,external_display_name,token_ciphertext,refresh_token_ciphertext,state) VALUES(?,?,'user','ou_test','Test User','token','refresh','active')", owner, enablement)
	exec("INSERT INTO cli_connector_authorization_attempts(owner_user_id,enablement_id,identity,device_code_ciphertext,action_url,expires_at) VALUES(?,?,'user','pending','https://accounts.feishu.cn/authorize',now()+interval '5 minutes')", owner, enablement)

	if _, err := repository.DisableCLIConnector(ctx, owner, definition, 2); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale disable = %v", err)
	}
	disabled, err := repository.DisableCLIConnector(ctx, owner, definition, 3)
	if err != nil || disabled.State != "disabled" || disabled.Version != 4 {
		t.Fatalf("disable = %#v, %v", disabled, err)
	}
	var applications, activeAuthorizations, pendingAuthorizations int64
	if err := db.Model(&feishuCLIApplicationRecord{}).Where("enablement_id = ?", enablement).Count(&applications).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&cliConnectorAuthorizationRecord{}).Where("enablement_id = ? AND state = 'active'", enablement).Count(&activeAuthorizations).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&cliConnectorAuthorizationAttemptRecord{}).Where("enablement_id = ?", enablement).Count(&pendingAuthorizations).Error; err != nil {
		t.Fatal(err)
	}
	if applications != 1 || activeAuthorizations != 1 || pendingAuthorizations != 0 {
		t.Fatalf("disable state: applications=%d active_authorizations=%d pending_authorizations=%d", applications, activeAuthorizations, pendingAuthorizations)
	}
	reenabled, err := repository.EnableCLIConnector(ctx, owner, definition)
	if err != nil || reenabled.ID != enablement || reenabled.State != "enabled" || reenabled.Version != 5 || reenabled.ProviderName != "Test" {
		t.Fatalf("reenable = %#v, %v", reenabled, err)
	}
}

func TestSessionCommandApprovalEntersUserActionWait(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	owner, session := uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", session, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO session_messages(session_id,role,content,state,progress_stage) VALUES(?,'assistant','','generating','using_tool')", session).Error; err != nil {
		t.Fatal(err)
	}
	var messageID int64
	if err := db.Raw("SELECT id FROM session_messages WHERE session_id=?", session).Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	digest := strings.Repeat("a", 64)
	go func() {
		_, err := repository.Await(ctx, cliconnector.ApprovalRequest{
			OwnerID: owner, ExecutionKind: "session", ExecutionID: fmt.Sprint(messageID), StageID: fmt.Sprintf("session:%d:stage:1", messageID),
			ConnectorName: "Feishu CLI", Operation: "im_messages_send", Target: "oc_test", RedactedArguments: "im +messages-send [arguments redacted]",
			CommandDigest: digest, Nonce: "nonce-1", Identity: cliconnector.IdentityUser, AllowedIdentities: []cliconnector.Identity{cliconnector.IdentityUser},
			CommandDigests: map[cliconnector.Identity]string{cliconnector.IdentityUser: digest}, ExpiresAt: time.Now().Add(time.Minute),
		})
		result <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		approvals, err := repository.ListCommandApprovals(context.Background(), owner, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(approvals) == 1 {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("approval wait stopped before becoming visible: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("approval did not become visible")
		}
		time.Sleep(20 * time.Millisecond)
	}

	var state, progress string
	if err := db.Raw("SELECT state,progress_stage FROM session_messages WHERE id=?", messageID).Row().Scan(&state, &progress); err != nil {
		t.Fatal(err)
	}
	if state != "waiting_for_user" || progress != "using_tool" {
		t.Fatalf("message state=%q progress=%q", state, progress)
	}
	job := application.ExecutionJob{Kind: application.JobSession, SessionID: session, AssistantMessageID: messageID}
	if err := repository.RecordProgress(context.Background(), job, application.ExecutionEvent{Type: "command.requested", Payload: []byte(`{"command":"im +messages-send"}`)}); err != nil {
		t.Fatalf("record Runtime command event while approval is pending: %v", err)
	}
	if err := db.Raw("SELECT state,progress_stage FROM session_messages WHERE id=?", messageID).Row().Scan(&state, &progress); err != nil {
		t.Fatal(err)
	}
	if state != "waiting_for_user" || progress != "using_tool" {
		t.Fatalf("Runtime event changed approval wait: state=%q progress=%q", state, progress)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled approval wait = %v", err)
	}
	if err := db.Raw("SELECT state,progress_stage FROM session_messages WHERE id=?", messageID).Row().Scan(&state, &progress); err != nil {
		t.Fatal(err)
	}
	if state != "generating" || progress != "using_tool" {
		t.Fatalf("resumed message state=%q progress=%q", state, progress)
	}
}

func TestSessionBrokerWaitsForTargetBoundCommandApproval(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	owner, session := uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", session, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO session_messages(session_id,role,content,state,progress_stage) VALUES(?,'assistant','','generating','using_tool')", session).Error; err != nil {
		t.Fatal(err)
	}
	var messageID int64
	if err := db.Raw("SELECT id FROM session_messages WHERE session_id=?", session).Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}
	definition := cliconnector.Definition{
		ID: "notion-1", Name: "Notion", Executable: "ntn", AuthenticationDriver: "none", State: cliconnector.StateAvailable,
		BundleSHA256: strings.Repeat("a", 64), RuntimeDigests: []string{"sha256:" + strings.Repeat("b", 64)},
		Capabilities: []cliconnector.Capability{{ID: "page-create", ArgvPrefix: []string{"pages", "create"}, Risk: cliconnector.RiskHigh, Identities: []cliconnector.Identity{cliconnector.IdentityUser}, EgressHosts: []string{"api.notion.com"}, Timeout: time.Minute}},
	}
	process := &approvalRecordingProcess{}
	broker, err := cliconnector.NewBroker(cliconnector.BrokerConfig{
		Definitions: []cliconnector.Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: cliconnector.Wrapper{Process: process},
		Approval: repository, ApprovalContext: cliconnector.ApprovalContext{OwnerID: owner, ExecutionKind: "session", ExecutionID: fmt.Sprint(messageID), StageID: fmt.Sprintf("session:%d:stage:1", messageID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	command := cliconnector.BrokerCommand{ConnectorID: definition.ID, Capability: "page-create", Identity: cliconnector.IdentityUser, Arguments: []string{"pages", "create", "--parent", "page:test-parent", "--content", "Test"}}
	if response := broker.Handle(context.Background(), command); response.ErrorCode != "invalid_request" {
		t.Fatalf("missing target response=%#v", response)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command.Target = "Test parent / test page"
	result := make(chan cliconnector.BrokerResponse, 1)
	go func() { result <- broker.Handle(ctx, command) }()
	var approval domain.CommandApproval
	deadline := time.Now().Add(3 * time.Second)
	for {
		approvals, err := repository.ListCommandApprovals(context.Background(), owner, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(approvals) == 1 {
			approval = approvals[0]
			break
		}
		select {
		case response := <-result:
			t.Fatalf("command stopped before approval appeared: %#v", response)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("command approval did not become visible")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var state string
	if err := db.Raw("SELECT state FROM session_messages WHERE id=?", messageID).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if approval.Target != command.Target || approval.Operation != command.Capability || state != "waiting_for_user" || process.starts.Load() != 0 {
		t.Fatalf("approval=%#v message_state=%q process_starts=%d", approval, state, process.starts.Load())
	}
	if _, err := repository.DecideCommandApproval(context.Background(), owner, approval.ID, domain.ApprovalApproved, domain.IdentityUser, approval.Version, time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		if response.ErrorCode != "" || response.ExitCode != 0 || process.starts.Load() != 1 {
			t.Fatalf("approved response=%#v process_starts=%d", response, process.starts.Load())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("approved command did not resume")
	}
	var approvalState string
	if err := db.Raw("SELECT state FROM cli_command_approvals WHERE id=?", approval.ID).Scan(&approvalState).Error; err != nil {
		t.Fatal(err)
	}
	if approvalState != "consumed" {
		t.Fatalf("approval state=%q", approvalState)
	}
}

type approvalRecordingProcess struct{ starts atomic.Int32 }

func (process *approvalRecordingProcess) Run(context.Context, cliconnector.ProcessRequest) (cliconnector.Result, error) {
	process.starts.Add(1)
	return cliconnector.Result{}, nil
}

func TestWorkflowRuntimeEventsPersistWhileApprovalIsPending(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	owner, workflowID, runID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','goal','workspace/test')", workflowID, owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot) VALUES(?,?,1,?,?,'Workflow','manual','waiting_for_user','{}'::jsonb)`, runID, runID, owner, workflowID).Error; err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: runID, OwnerID: owner}
	if err := repository.RecordProgress(context.Background(), job, application.ExecutionEvent{Type: "command.requested", Payload: []byte(`{"command":"im +messages-send"}`)}); err != nil {
		t.Fatalf("record Workflow Runtime event while approval is pending: %v", err)
	}
	var count int64
	if err := db.Table("run_events").Where("run_id = ? AND event_type = 'command.requested'", runID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("recorded Workflow Runtime events = %d, want 1", count)
	}
}

func TestCLIConnectorDeletionRevokesAccessAndPreservesHistory(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, expert, enablement, session := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner)
	input := cliconnector.Definition{Name: "Feishu CLI", Icon: "terminal", Description: "Read Feishu", InstallationType: "npm", Package: "@larksuite/cli", Version: "1.0.93", AuthenticationDriver: "feishu"}
	item, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", item.ID)
	exec("INSERT INTO cli_connector_enablements(id,owner_user_id,definition_id,state,registration_device_code_ciphertext) VALUES(?,?,?,'enabled','test-device')", enablement, owner, item.ID)
	exec("INSERT INTO cli_connector_authorizations(owner_user_id,enablement_id,identity,external_identity_id,external_display_name,token_ciphertext,refresh_token_ciphertext,state) VALUES(?,?,'user','test-user','Test','test-token','test-refresh','active')", owner, enablement)
	exec("INSERT INTO cli_connector_authorization_attempts(owner_user_id,enablement_id,identity,device_code_ciphertext,action_url,expires_at) VALUES(?,?,'user','test-device','https://example.test',now()+interval '5 minutes')", owner, enablement)
	exec("INSERT INTO feishu_cli_applications(owner_user_id,enablement_id,provider_application_id_ciphertext,provider_application_secret_ciphertext,provider_name,developer_console_url) VALUES(?,?,'test-app','test-secret','Test','https://example.test')", owner, enablement)
	exec("INSERT INTO experts(id,owner_user_id,name,cli_connector_definition_ids) VALUES(?,?,'Test',jsonb_build_array(?::text))", expert, owner, item.ID)
	exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", session, owner)
	snapshot := `{"stages":[{"cli_connectors":[{"id":"` + item.ID + `"}]}]}`
	exec("INSERT INTO session_messages(session_id,role,content,state,response_snapshot) VALUES(?,'assistant','Historical result','completed',?::jsonb)", session, snapshot)
	if err := repository.DeleteCLIConnectorDefinition(ctx, item.ID, item.VersionNumber+1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale deletion = %v", err)
	}
	if _, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID); err != nil {
		t.Fatal("stale deletion revoked access")
	}
	if err := repository.DeleteCLIConnectorDefinition(ctx, item.ID, item.VersionNumber); err != nil {
		t.Fatal(err)
	}
	for _, unpublished := range []bool{false, true} {
		items, err := repository.ListCLIConnectorDefinitions(ctx, unpublished)
		if err != nil || len(items) != 0 {
			t.Fatalf("deleted definition remains in catalog: %v", err)
		}
	}
	if _, err := repository.GetAvailableCLIConnectorDefinition(ctx, item.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted connector is available: %v", err)
	}
	if _, err := repository.UpdateCLIConnectorDefinition(ctx, item.ID, input, item.VersionNumber+1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("deleted connector can be edited: %v", err)
	}
	if _, err := repository.PublishCLIConnectorDefinition(ctx, item.ID, item.VersionNumber+1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted connector can be published: %v", err)
	}
	if values, err := repository.ListCLIConnectorEnablements(ctx, owner); err != nil || len(values) != 0 {
		t.Fatalf("deleted enablement visible: %v", err)
	}
	if values, err := repository.ListCLIConnectorHealth(ctx, time.Now()); err != nil || len(values) != 0 {
		t.Fatalf("deleted health visible: %v", err)
	}
	var count int64
	checks := []string{
		"SELECT count(*) FROM cli_connector_enablements WHERE state <> 'disabled' OR registration_device_code_ciphertext IS NOT NULL",
		"SELECT count(*) FROM cli_connector_authorizations WHERE state <> 'disconnected' OR token_ciphertext IS NOT NULL OR refresh_token_ciphertext IS NOT NULL",
		"SELECT count(*) FROM cli_connector_authorization_attempts",
		"SELECT count(*) FROM experts WHERE cli_connector_definition_ids <> '[]'::jsonb OR version <> 2",
	}
	for _, query := range checks {
		if err := db.Raw(query).Scan(&count).Error; err != nil || count != 0 {
			t.Fatalf("deletion cleanup failed for %s: count=%d err=%v", query, count, err)
		}
	}
	if err := db.Raw("SELECT count(*) FROM session_messages WHERE response_snapshot=?::jsonb", snapshot).Scan(&count).Error; err != nil || count != 1 {
		t.Fatal("historical snapshot changed")
	}
	if err := db.Raw("SELECT count(*) FROM feishu_cli_applications").Scan(&count).Error; err != nil || count != 1 {
		t.Fatal("User's Feishu Application was removed")
	}
	replacement, err := repository.CreateCLIConnectorDefinition(ctx, owner, input)
	if err != nil {
		t.Fatalf("cannot install the same package after deletion: %v", err)
	}
	exec("UPDATE cli_connector_definitions SET state='available' WHERE id=?", replacement.ID)
	restored, err := repository.EnableCLIConnector(ctx, owner, replacement.ID)
	if err != nil || restored.State != "enabled" || restored.ProviderName != "Test" {
		t.Fatalf("existing Feishu Application was not reused: %v", err)
	}
	if _, err := repository.GetFeishuCLIApplicationCredentials(ctx, owner, restored.ID); err != nil {
		t.Fatalf("restored credentials unavailable: %v", err)
	}
	again, err := repository.EnableCLIConnector(ctx, owner, replacement.ID)
	if err != nil || again.ID != restored.ID || again.Version != restored.Version {
		t.Fatalf("enablement is not idempotent: %v", err)
	}
	if _, err := repository.EnableCLIConnector(ctx, uuid.NewString(), replacement.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another User reused the application: %v", err)
	}
}
