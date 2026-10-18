package runtimeexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/biz/workspace/domain"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func coordinationPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKSPACE_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })
	name := "coordination_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(`CREATE DATABASE "` + name + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`).Error; err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + name
	db, err := gorm.Open(postgres.Open(parsed.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCoordinatedExecutorSettlesEachActualCallBeforeNextBudgetAdmission(t *testing.T) {
	db := coordinationPostgres(t)
	executor, job, _ := coordinatedFixture(t)
	job.OwnerID = uuid.NewString()
	job.Timezone = "Asia/Shanghai"
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", job.OwnerID, job.OwnerID, job.OwnerID, job.OwnerID+"@example.test", job.OwnerID).Error; err != nil {
		t.Fatal(err)
	}
	creditRepository := creditsrepo.New(db)
	service, err := creditsapplication.New(creditRepository, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.EnableCredits(service); err != nil {
		t.Fatal(err)
	}
	rate, err := creditRepository.ResolveRate(context.Background(), creditsdomain.ModelRateKey{ProviderType: "openai", Protocol: "openai_responses", ModelID: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	secret, err := executor.box.Encrypt([]byte("test-key"), "model-provider:"+job.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range job.Snapshot.Stages {
		job.Snapshot.Stages[i].ProviderModel.ProviderType = "openai"
		job.Snapshot.Stages[i].ProviderModel.Protocols = []string{"openai_responses"}
		job.Snapshot.Stages[i].ProviderModel.APIKeyCiphertext = secret
		job.Snapshot.Stages[i].CreditRate = &domain.CreditRateSnapshot{RevisionID: rate.RevisionID, InputMultiplierMicros: rate.InputMultiplierMicros, OutputMultiplierMicros: rate.OutputMultiplierMicros, FallbackHundredths: 50}
	}
	job.Snapshot.Coordination.CreditBudgetHundredths = 100
	metadata, _ := json.Marshal(job.Snapshot)
	if err := db.Exec(`INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Team','Goal',?)`, job.WorkflowID, job.OwnerID, job.Snapshot.WorkspacePath).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot) VALUES(?,?,1,?,?,'Team','manual','running',?::jsonb)`, job.ID, job.ID, job.OwnerID, job.WorkflowID, string(metadata)).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			calls++
			value := coordinationRuntimeResult(t, sink, request.RunID, `{"action":"delegate","tasks":[{"id":"review","member_id":"reviewer","instruction":"Review","required":true}]}`)
			value.Usage = agentruntime.Usage{InputTokens: 20000, Reported: true}
			return value, nil
		}}, nil
	}
	repository := workspacerepo.New(db, creditRepository)
	result, err := executor.Execute(context.Background(), job, repository)
	if !errors.Is(err, creditsdomain.ErrResponseBudgetExhausted) || calls != 1 || result.SuccessCommit != nil {
		t.Fatalf("admission was bypassed: calls=%d err=%v", calls, err)
	}
	if result.CreditConsumption == nil || result.CreditConsumption.TotalHundredths != 200 {
		t.Fatalf("actual charge=%+v", result.CreditConsumption)
	}
	if err := repository.FinishFailed(context.Background(), job, result, "Response admission budget exhausted"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Table("credit_ledger").Where("user_id = ? AND entry_type = 'consumption'", job.OwnerID).Count(&count)
	if count != 1 {
		t.Fatalf("invocation settled %d times", count)
	}
	var admitted int64
	db.Table("credit_stage_admissions").Where("user_id = ?", job.OwnerID).Count(&admitted)
	if admitted != 1 {
		t.Fatal("unstarted member left a Credit reservation")
	}
}
