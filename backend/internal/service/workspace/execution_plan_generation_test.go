package workspace

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	aiapp "agent-platform/backend/internal/biz/aiapplication/application"
	creditsapp "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspaceapp "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
)

type planModelRepository struct {
	workspaceapp.Repository
	credential []byte
	connection workspacedomain.ModelProviderConnection
}

func (repository *planModelRepository) ListModelProviderConnections(context.Context) ([]workspacedomain.ModelProviderConnection, error) {
	return []workspacedomain.ModelProviderConnection{repository.connection}, nil
}
func (repository *planModelRepository) GetSettings(context.Context, string) (workspacedomain.Settings, error) {
	return workspacedomain.Settings{Timezone: "Asia/Shanghai"}, nil
}
func (repository *planModelRepository) GetModelProviderAPIKey(context.Context, string, string) ([]byte, error) {
	return repository.credential, nil
}

type planCreditRepository struct {
	creditsapp.Repository
	admitted, settled bool
}

func (repository *planCreditRepository) ResolveRate(context.Context, creditsdomain.ModelRateKey) (creditsdomain.ModelCreditRate, error) {
	return creditsdomain.ModelCreditRate{RevisionID: "rate", InputMultiplierMicros: 1_000_000, OutputMultiplierMicros: 1_000_000, Fallback: 100}, nil
}
func (repository *planCreditRepository) Admit(_ context.Context, admission creditsdomain.Admission) (creditsdomain.Admission, error) {
	repository.admitted = true
	return admission, nil
}
func (repository *planCreditRepository) Settle(_ context.Context, settlement creditsdomain.Settlement) (creditsdomain.Consumption, error) {
	repository.settled = true
	return creditsdomain.Consumption{Amount: 37, Usage: settlement.Usage}, nil
}

type planChatModel struct {
	calls   int
	request aiapp.ChatRequest
}

func (model *planChatModel) Generate(_ context.Context, request aiapp.ChatRequest, _ func(string) error) (aiapp.ChatResult, error) {
	model.calls++
	model.request = request
	return aiapp.ChatResult{Text: `{"labels":["确认代码分析范围和入口","梳理目录并追踪核心调用链","汇总鉴权风险与建议"]}`, InputTokens: 100, OutputTokens: 30, UsageKnown: true}, nil
}

func TestParseModelPlanLabelsRequiresTaskSpecificExactSteps(t *testing.T) {
	labels, err := parseModelPlanLabels(`{"labels":["确认代码分析范围和入口","梳理目录并追踪核心调用链","汇总鉴权风险与建议"]}`, 3)
	if err != nil || len(labels) != 3 || labels[1] != "梳理目录并追踪核心调用链" {
		t.Fatalf("parse specific Plan: %q, %v", labels, err)
	}
	for _, value := range []string{
		`{"labels":["核对输入","执行任务","交付结果"]}`,
		`{"labels":["确认范围","梳理代码"]}`,
		`{"labels":["确认范围","梳理代码","汇总结果"]} trailing`,
		`{"labels":["确认范围","梳理代码","汇总结果"],"extra":true}`,
		`{"labels":["确认代码范围","确认代码范围","汇总分析结论"]}`,
	} {
		if _, err := parseModelPlanLabels(value, 3); err == nil {
			t.Fatalf("accepted invalid Plan model output: %s", value)
		}
	}
}

func TestGeneratePlanLabelsUsesFrozenProviderModelAndSettlesCredits(t *testing.T) {
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	credential, err := box.Encrypt([]byte("test-model-secret"), "model-provider:admin")
	if err != nil {
		t.Fatal(err)
	}
	repository := &planModelRepository{credential: credential, connection: workspacedomain.ModelProviderConnection{
		ID: "connection", CredentialOwnerID: "admin", Version: 3, HasAPIKey: true,
	}}
	workspace, err := workspaceapp.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	creditRepository := &planCreditRepository{}
	credits, err := creditsapp.New(creditRepository, nil)
	if err != nil {
		t.Fatal(err)
	}
	model := &planChatModel{}
	service := &Service{workspace: workspace, credits: credits, box: box, assistantChatModel: model}
	plan, err := workspacedomain.BuildExecutionPlan(workspacedomain.ExecutionPlanContext{
		Objective: "分析项目代码", Preference: workspacedomain.PlanPreferenceAlways,
		Stages: []workspacedomain.ExecutionStageSnapshot{{Position: 1}},
	}, time.Now())
	if err != nil || plan == nil {
		t.Fatalf("build Plan: %#v, %v", plan, err)
	}
	stage := workspacedomain.ExecutionStageSnapshot{Position: 1, ModelProtocol: "openai_responses", ProviderModel: workspacedomain.ProviderModelSnapshot{
		ID: "model", ConnectionID: "connection", ConnectionVersion: 3, ProviderType: "openai", ModelID: "gpt-test", Endpoint: "https://example.test/v1",
	}}
	labels, cost, err := service.generatePlanLabels(context.Background(), "owner", "session-plan-1", plan, []workspacedomain.ExecutionStageSnapshot{stage})
	if err != nil || cost != 37 || len(labels) != 3 || !creditRepository.admitted || !creditRepository.settled || model.calls != 1 {
		t.Fatalf("model Plan labels=%q cost=%d err=%v admitted=%v settled=%v calls=%d", labels, cost, err, creditRepository.admitted, creditRepository.settled, model.calls)
	}
	if len(model.request.APIKey) == 0 || model.request.ModelID != "gpt-test" || model.request.Protocol != "openai_responses" {
		t.Fatal("wrong frozen Provider Model request")
	}
	stage.ProviderModel.ConnectionVersion = 2
	if _, _, err := service.generatePlanLabels(context.Background(), "owner", "session-plan-2", plan, []workspacedomain.ExecutionStageSnapshot{stage}); err == nil || model.calls != 1 {
		t.Fatalf("stale Provider Model credential was used: %v, calls=%d", err, model.calls)
	}
}

func TestAutomaticSafetyPlanDoesNotCallModel(t *testing.T) {
	service := &Service{}
	plan, err := workspacedomain.BuildExecutionPlan(workspacedomain.ExecutionPlanContext{
		Objective: "分析项目代码", Preference: workspacedomain.PlanPreferenceAuto,
		Workflow: true, Stages: []workspacedomain.ExecutionStageSnapshot{{Position: 1}},
	}, time.Now())
	if err != nil || plan == nil {
		t.Fatalf("build automatic safety Plan: %#v, %v", plan, err)
	}
	run := workspacedomain.Run{ExecutionPlan: plan}
	got, err := service.completeRunPlanGeneration(context.Background(), nil, "owner", "workflow", run, workspacedomain.PlanPreferenceAuto)
	if err != nil || got.ExecutionPlan != plan || got.ExecutionPlan.Generator != "platform_rules" {
		t.Fatalf("automatic safety Plan triggered model generation: %#v, %v", got.ExecutionPlan, err)
	}
}
