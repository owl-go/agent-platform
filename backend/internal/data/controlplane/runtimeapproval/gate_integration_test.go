package runtimeapproval_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/platformprotocol"
	"agent-platform/backend/internal/agentruntime/processharness"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/biz/transaction"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/data/controlplane/gormuow"
	"agent-platform/backend/internal/data/controlplane/runtimeapproval"
	executiongorm "agent-platform/backend/internal/data/execution/gormrepo"
	"agent-platform/backend/internal/data/execution/runtimeprocessor"
	"agent-platform/backend/internal/infrastructure/gormdb"
)

func TestRuntimeApprovalBrowserClosedLoop(t *testing.T) {
	if os.Getenv("GO_WANT_RUNTIME_APPROVAL_HELPER") == "1" {
		runtimeApprovalHelperProcess()
		return
	}
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	runID := os.Getenv("RUNTIME_APPROVAL_BROWSER_RUN_ID")
	readyFile := os.Getenv("RUNTIME_APPROVAL_READY_FILE")
	markerFile := os.Getenv("RUNTIME_APPROVAL_MARKER_FILE")
	if dsn == "" || runID == "" || readyFile == "" || markerFile == "" {
		t.Skip("Runtime Approval browser integration environment is required")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runs := executionapplication.New(executiongorm.New(database.ORM()))
	lease, found, err := runs.Claim(context.Background(), "runtime-approval-browser", time.Minute)
	if err != nil || !found || lease.RunID != runID {
		t.Fatalf("Claim() = (%+v, %v, %v), want Run %s", lease, found, err, runID)
	}
	if err := runs.MarkRunning(context.Background(), lease.Token); err != nil {
		t.Fatal(err)
	}
	writes := gormuow.NewWithWebhook(database.ORM(), "https://hooks.example.test/runtime-approval-browser")
	gate, err := runtimeapproval.New(database.ORM(), writes)
	if err != nil {
		t.Fatal(err)
	}
	processor, err := runtimeprocessor.New(runs, integrationResolver{}, credentials.Materializer{Root: t.TempDir()}, integrationRuntimeFactory{markerFile: markerFile, workspaceDirectory: t.TempDir()}, gate)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		outcome, executeErr := processor.Execute(context.Background(), lease)
		if executeErr == nil {
			executeErr = runs.Finish(context.Background(), lease.Token, outcome)
		}
		done <- executeErr
	}()
	select {
	case err := <-done:
		t.Fatalf("Runtime returned before requesting Approval: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	approval := waitForPendingApproval(t, database, runID)
	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(markerFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Runtime crossed protected boundary while Approval was pending: %v", err)
	}
	if err := os.WriteFile(readyFile, []byte(approval.ID), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Runtime did not resume after browser Approval")
	}
	if _, err := os.Stat(markerFile); err != nil {
		t.Fatalf("approved Runtime did not cross protected boundary: %v", err)
	}
	var state string
	if err := database.ORM().Raw(`SELECT state FROM runs WHERE id = ?`, runID).Scan(&state).Error; err != nil || state != "completed" {
		t.Fatalf("Run state = %q, error=%v", state, err)
	}
}

func TestRuntimeApprovalGateUsesAtomicGovernanceAndClosesAbandonedApproval(t *testing.T) {
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	runID := os.Getenv("RUNTIME_APPROVAL_RUN_ID")
	if dsn == "" || runID == "" {
		t.Skip("EXECUTION_DATABASE_DSN and RUNTIME_APPROVAL_RUN_ID are required")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	writes := gormuow.NewWithWebhook(database.ORM(), "https://hooks.example.test/runtime-approval")
	gate, err := runtimeapproval.New(database.ORM(), writes)
	if err != nil {
		t.Fatal(err)
	}

	firstDone := make(chan error, 1)
	firstRequest := executionapplication.RuntimeApprovalRequest{
		RunID: runID, AttemptID: "integration-attempt-1", Sequence: 2, Kind: "high_risk_change", Request: json.RawMessage(`{"risk_reason":"integration protected write"}`),
	}
	go func() {
		firstDone <- gate.AwaitDecision(context.Background(), firstRequest)
	}()
	select {
	case err := <-firstDone:
		t.Fatalf("Runtime Approval request returned before decision: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	first := waitForPendingApproval(t, database, runID)
	decideApproval(t, writes, first, true)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := gate.AwaitDecision(context.Background(), firstRequest); err != nil {
		t.Fatalf("committed Runtime Approval replay = %v", err)
	}

	waitCtx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- gate.AwaitDecision(waitCtx, executionapplication.RuntimeApprovalRequest{
			RunID: runID, AttemptID: "integration-attempt-2", Sequence: 2, Kind: "plan", Request: json.RawMessage(`{"summary":"abandoned integration plan"}`),
		})
	}()
	second := waitForPendingApproval(t, database, runID)
	if second.ID == first.ID {
		t.Fatal("second Runtime Approval reused the first identity")
	}
	cancel()
	if err := <-secondDone; !errors.Is(err, executiondomain.ErrApprovalRejected) {
		t.Fatalf("abandoned Runtime Approval error = %v", err)
	}

	var pending int64
	if err := database.ORM().Raw(`SELECT count(*) FROM approvals WHERE run_id = ? AND state = 'pending'`, runID).Scan(&pending).Error; err != nil || pending != 0 {
		t.Fatalf("pending Approvals = %d, error=%v", pending, err)
	}
	var evidence struct {
		Audit          int64 `gorm:"column:audit_count"`
		Webhook        int64 `gorm:"column:webhook_count"`
		Idempotency    int64 `gorm:"column:idempotency_count"`
		Terminals      int64 `gorm:"column:terminal_count"`
		SystemDecision int64 `gorm:"column:system_decision_count"`
		SystemAudit    int64 `gorm:"column:system_audit_count"`
	}
	query := `SELECT
		(SELECT count(*) FROM audit_events WHERE resource_id IN (?, ?)) AS audit_count,
		(SELECT count(*) FROM webhook_deliveries WHERE target_url = 'https://hooks.example.test/runtime-approval') AS webhook_count,
		(SELECT count(*) FROM idempotency_keys WHERE operation IN (?, ?, ?)) AS idempotency_count,
		(SELECT count(*) FROM run_events WHERE run_id = ? AND event_type = 'run.failed') AS terminal_count,
		(SELECT count(*) FROM approvals WHERE id = ? AND decision_actor_type = 'system' AND decided_by IS NULL) AS system_decision_count,
		(SELECT count(*) FROM audit_events WHERE resource_id = ? AND actor_user_id IS NULL AND details->>'actor_type' = 'system') AS system_audit_count`
	if err := database.ORM().Raw(query, first.ID, second.ID, "approval.request:"+runID, "approval.decide:"+first.ID, "approval.decide:"+second.ID, runID, second.ID, second.ID).Scan(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.Audit < 4 || evidence.Webhook < 4 || evidence.Idempotency < 4 || evidence.Terminals != 1 || evidence.SystemDecision != 1 || evidence.SystemAudit != 1 {
		t.Fatalf("governance evidence = %+v", evidence)
	}
}

type approvalRecord struct {
	ID             string
	RunID          string
	OrganizationID string
	TeamID         string
	ActorID        string
	Version        int64
}

func waitForPendingApproval(t *testing.T, database *gormdb.Database, runID string) approvalRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var approval approvalRecord
		err := database.ORM().Raw(`SELECT approval.id::text, approval.run_id::text, task.organization_id::text, task.team_id::text,
			approval.requested_by::text AS actor_id, approval.version
			FROM approvals approval JOIN runs run ON run.id = approval.run_id
			JOIN sessions session ON session.id = run.session_id JOIN coding_tasks task ON task.id = session.coding_task_id
			WHERE approval.run_id = ? AND approval.state = 'pending' ORDER BY approval.requested_at DESC LIMIT 1`, runID).Scan(&approval).Error
		if err == nil && approval.ID != "" {
			return approval
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("Runtime Approval did not become pending")
	return approvalRecord{}
}

func decideApproval(t *testing.T, writes transaction.IdempotentTransactionManager, approval approvalRecord, approved bool) {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"approved":%t,"version":%d}`, approved, approval.Version))
	digest := sha256.Sum256(body)
	_, err := writes.Execute(context.Background(), transaction.IdempotencyRequest{
		OrganizationID: approval.OrganizationID, TeamID: approval.TeamID, ActorUserID: approval.ActorID,
		Key: "runtime-integration-decision:" + approval.ID, Operation: "approval.decide:" + approval.ID,
		RequestSHA256: hex.EncodeToString(digest[:]), ExpiresAt: time.Now().Add(time.Hour),
	}, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		updated, decideErr := services.Approvals.Decide(context.Background(), approval.ID, approval.Version, approved, approval.ActorID, "integration decision")
		response, _ := json.Marshal(map[string]any{"id": updated.ID, "state": updated.State})
		return transaction.IdempotencyResult{Status: http.StatusOK, Body: response}, decideErr
	})
	if err != nil {
		t.Fatal(err)
	}
}

type integrationResolver struct{}

func (integrationResolver) Resolve(context.Context, string, []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
	return credentials.Request{Ref: "runtime-approval-browser", Variables: map[string]string{"MODEL_API_KEY": "integration-secret"}}, nil
}

type integrationRuntimeFactory struct {
	markerFile         string
	workspaceDirectory string
}

func (factory integrationRuntimeFactory) New(lease executiondomain.Lease, _ runtimeprocessor.Plan, _ *credentials.Environment) (agentruntime.Adapter, error) {
	return cliadapter.New(integrationDriver{name: lease.RuntimeName, markerFile: factory.markerFile}, cliadapter.Config{
		Command:         []string{"env", "GO_WANT_RUNTIME_APPROVAL_HELPER=1", "RUNTIME_APPROVAL_CLI_VERSION=" + lease.RuntimeCLIVersion, os.Args[0], "-test.run=TestRuntimeApprovalBrowserClosedLoop", "--"},
		ExpectedVersion: lease.RuntimeCLIVersion,
		RunProcess: func(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (processharness.Result, error) {
			spec.Dir = factory.workspaceDirectory
			return processharness.Run(ctx, spec, sink)
		},
	}), nil
}

type integrationDriver struct {
	name       string
	markerFile string
}

func (driver integrationDriver) Name() string   { return driver.name }
func (integrationDriver) VersionArgs() []string { return []string{"--version"} }
func (integrationDriver) ParseVersion(value string) (string, error) {
	return strings.TrimSpace(value), nil
}
func (driver integrationDriver) Build(agentruntime.ExecuteRequest, string) (cliadapter.Invocation, error) {
	return cliadapter.Invocation{Args: []string{"execute"}, Env: []string{"RUNTIME_APPROVAL_MARKER_FILE=" + driver.markerFile}}, nil
}
func (integrationDriver) NewParser(string) cliadapter.Parser { return &integrationParser{} }

type integrationParser struct{ result cliadapter.ParsedResult }

func (parser *integrationParser) Parse(_ processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	var value struct {
		Approval json.RawMessage `json:"approval"`
		Final    string          `json:"final"`
	}
	if err := json.Unmarshal(line, &value); err != nil {
		return nil, err
	}
	if len(value.Approval) > 0 {
		return []cliadapter.ParsedEvent{{Kind: agentruntime.EventApprovalRequested, Payload: json.RawMessage(value.Approval)}}, nil
	}
	if value.Final != "" {
		parser.result.FinalMessage = value.Final
	}
	return nil, nil
}

func (parser *integrationParser) Result() cliadapter.ParsedResult { return parser.result }

func runtimeApprovalHelperProcess() {
	args := os.Args
	mode := args[len(args)-1]
	if mode == "--version" {
		fmt.Fprintln(os.Stdout, os.Getenv("RUNTIME_APPROVAL_CLI_VERSION"))
		os.Exit(0)
	}
	if mode != "execute" {
		os.Exit(2)
	}
	line, err := platformprotocol.EncodeApprovalRequest("Browser protected write", "agent-platform/browser")
	if err != nil {
		os.Exit(3)
	}
	fmt.Fprintln(os.Stdout, string(line))
	if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv("RUNTIME_APPROVAL_MARKER_FILE"), []byte("continued"), 0o600); err != nil {
		os.Exit(3)
	}
	fmt.Fprintln(os.Stdout, `{"final":"completed after Approval"}`)
	os.Exit(0)
}
