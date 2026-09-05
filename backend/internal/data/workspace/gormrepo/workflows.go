package gormrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) ListWorkflows(ctx context.Context, ownerID string, deleted bool) ([]domain.Workflow, error) {
	query := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID)
	if deleted {
		query = query.Where("deleted_at IS NOT NULL")
	} else {
		query = query.Where("deleted_at IS NULL")
	}
	var rows []workflowRecord
	if err := query.Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Workflows: %w", err)
	}
	items := make([]domain.Workflow, 0, len(rows))
	for _, row := range rows {
		item, err := workflowDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateWorkflow(ctx context.Context, ownerID string, input domain.WorkflowInput, secretCiphertext []byte) (domain.Workflow, error) {
	if err := input.Validate(); err != nil {
		return domain.Workflow{}, err
	}
	id := uuid.NewString()
	row, err := workflowRecordForInput(id, ownerID, "workflows/"+ownerID+"/"+id, input, secretCiphertext)
	if err != nil {
		return domain.Workflow{}, err
	}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateWorkflowReferences(tx, ownerID, input); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		return domain.Workflow{}, fmt.Errorf("create Workflow: %w", err)
	}
	return workflowDomain(row)
}

func (repository *Repository) GetWorkflow(ctx context.Context, ownerID, workflowID string, includeDeleted bool) (domain.Workflow, error) {
	query := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, workflowID)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	var row workflowRecord
	if err := query.Take(&row).Error; err != nil {
		return domain.Workflow{}, mapNotFound(err)
	}
	return workflowDomain(row)
}

func (repository *Repository) UpdateWorkflow(ctx context.Context, ownerID, workflowID string, input domain.WorkflowInput, secretCiphertext []byte, expectedVersion int64) (domain.Workflow, error) {
	if err := input.Validate(); err != nil {
		return domain.Workflow{}, err
	}
	environment, err := marshal(input.Environment)
	if err != nil {
		return domain.Workflow{}, err
	}
	schedule, err := marshalNullable(input.Schedule)
	if err != nil {
		return domain.Workflow{}, err
	}
	updates := map[string]any{
		"name": strings.TrimSpace(input.Name), "goal": strings.TrimSpace(input.Goal), "expert_id": input.ExpertID, "expert_team_id": input.ExpertTeamID,
		"provider_model_id": nil, "runtime_engine": nil, "environment": environment,
		"schedule": schedule, "next_scheduled_at": nextScheduledAt(input.Schedule, time.Now().UTC()), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
	}
	if secretCiphertext != nil {
		updates["environment_secret_ciphertext"] = secretCiphertext
	}
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateWorkflowReferences(tx, ownerID, input); err != nil {
			return err
		}
		result := tx.Model(&workflowRecord{}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL AND version = ?", ownerID, workflowID, expectedVersion).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.Workflow{}, fmt.Errorf("update Workflow: %w", err)
	}
	return repository.GetWorkflow(ctx, ownerID, workflowID, false)
}

func validateWorkflowReferences(tx *gorm.DB, ownerID string, input domain.WorkflowInput) error {
	checks := []struct {
		value *string
		model any
		name  string
		where string
	}{
		{value: input.ExpertID, model: &expertRecord{}, name: "Expert", where: "owner_user_id = ? AND id = ?"},
		{value: input.ExpertTeamID, model: &expertTeamRecord{}, name: "Expert Team", where: "owner_user_id = ? AND id = ?"},
	}
	for _, check := range checks {
		if check.value == nil {
			continue
		}
		if _, err := uuid.Parse(*check.value); err != nil {
			return fmt.Errorf("%w: invalid %s identifier", domain.ErrInvalid, check.name)
		}
		var count int64
		if err := tx.Model(check.model).Where(check.where, ownerID, *check.value).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("%w: selected %s does not belong to the User", domain.ErrInvalid, check.name)
		}
	}
	if input.ExpertTeamID != nil {
		var team expertTeamRecord
		if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, *input.ExpertTeamID).Take(&team).Error; err != nil {
			return fmt.Errorf("%w: selected Expert Team does not belong to the User", domain.ErrInvalid)
		}
		var ids []string
		if err := json.Unmarshal(team.ExpertIDs, &ids); err != nil {
			return err
		}
		if len(ids) < 2 {
			return fmt.Errorf("%w: selected Expert Team requires at least two Experts", domain.ErrInvalid)
		}
		var available int64
		if err := tx.Model(&expertRecord{}).Where("owner_user_id = ? AND id IN ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", ownerID, ids).Count(&available).Error; err != nil {
			return err
		}
		if available != int64(len(ids)) {
			return fmt.Errorf("%w: selected Expert Team contains an unavailable Expert", domain.ErrInvalid)
		}
	}
	return nil
}

func (repository *Repository) DeleteWorkflow(ctx context.Context, ownerID, workflowID string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&workflowRecord{}).
			Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).
			Updates(map[string]any{
				"deleted_at": now, "updated_at": now, "version": gorm.Expr("version + 1"),
				"goal": "", "expert_id": nil, "expert_team_id": nil, "provider_model_id": nil, "runtime_engine": nil,
				"environment": []byte(`[]`), "environment_secret_ciphertext": nil,
				"api_key": nil, "api_secret_hash": nil, "schedule": nil, "next_scheduled_at": nil,
				"git_source": nil, "git_secret_ciphertext": nil,
			})
		if result.Error != nil {
			return fmt.Errorf("delete Workflow: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		var queued []runRecord
		if err := tx.Where("owner_user_id = ? AND workflow_id = ? AND state = 'queued'", ownerID, workflowID).Find(&queued).Error; err != nil {
			return err
		}
		if err := tx.Model(&runRecord{}).Where("owner_user_id = ? AND workflow_id = ? AND state = 'queued'", ownerID, workflowID).
			Updates(map[string]any{"state": "cancelled", "cancel_requested_at": now, "ended_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		for _, run := range queued {
			if err := appendRunEvents(tx, run.ID, nil, "run.cancelled", now); err != nil {
				return err
			}
		}
		return tx.Model(&runRecord{}).Where("owner_user_id = ? AND workflow_id = ? AND state = 'running'", ownerID, workflowID).
			Update("cancel_requested_at", now).Error
	})
}

func (repository *Repository) SetWorkflowCredential(ctx context.Context, ownerID, workflowID, apiKey, secretHash string) (domain.Workflow, error) {
	result := repository.db.WithContext(ctx).Model(&workflowRecord{}).
		Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).
		Updates(map[string]any{"api_key": apiKey, "api_secret_hash": secretHash, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return domain.Workflow{}, fmt.Errorf("set Workflow API credential: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.Workflow{}, domain.ErrNotFound
	}
	return repository.GetWorkflow(ctx, ownerID, workflowID, false)
}

func (repository *Repository) ResolveWorkflowCredential(ctx context.Context, workflowID, apiKey string) (string, string, error) {
	var row struct {
		OwnerID    string `gorm:"column:owner_user_id"`
		SecretHash string `gorm:"column:api_secret_hash"`
	}
	err := repository.db.WithContext(ctx).Table("workflows").
		Select("workflows.owner_user_id, workflows.api_secret_hash").
		Joins("JOIN users owner ON owner.id = workflows.owner_user_id AND owner.disabled_at IS NULL").
		Where("workflows.id = ? AND workflows.api_key = ? AND workflows.api_secret_hash IS NOT NULL AND workflows.deleted_at IS NULL", workflowID, apiKey).
		Take(&row).Error
	if err != nil {
		return "", "", mapNotFound(err)
	}
	return row.OwnerID, row.SecretHash, nil
}

func (repository *Repository) GetWorkflowEnvironmentSecret(ctx context.Context, ownerID, workflowID string) ([]byte, error) {
	var row workflowRecord
	if err := repository.db.WithContext(ctx).Select("environment_secret_ciphertext").Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return append([]byte(nil), row.EnvironmentSecret...), nil
}

func (repository *Repository) SetWorkflowGitSource(ctx context.Context, ownerID, workflowID string, source domain.GitSource, secretCiphertext []byte) (domain.Workflow, error) {
	if err := domain.ValidateGitSource(source); err != nil {
		return domain.Workflow{}, err
	}
	encoded, err := marshal(source)
	if err != nil {
		return domain.Workflow{}, err
	}
	result := repository.db.WithContext(ctx).Model(&workflowRecord{}).
		Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).
		Updates(map[string]any{"git_source": encoded, "git_secret_ciphertext": secretCiphertext, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return domain.Workflow{}, fmt.Errorf("set Workflow Git source: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.Workflow{}, domain.ErrNotFound
	}
	return repository.GetWorkflow(ctx, ownerID, workflowID, false)
}

func (repository *Repository) CreateRun(ctx context.Context, ownerID, workflowID, trigger string, textInput *string, jsonInput map[string]any) (domain.Run, error) {
	if trigger != "manual" && trigger != "scheduled" && trigger != "api" {
		return domain.Run{}, fmt.Errorf("%w: invalid Run trigger", domain.ErrInvalid)
	}
	if err := validateRunInput(textInput, jsonInput); err != nil {
		return domain.Run{}, err
	}
	var created runRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		created, err = createRunOnTx(tx, ownerID, workflowID, trigger, textInput, jsonInput)
		return err
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("create Workflow Run: %w", err)
	}
	return runDomain(created), nil
}

func (repository *Repository) CreateRunIdempotent(ctx context.Context, ownerID, workflowID, trigger, key string, textInput *string, jsonInput map[string]any) (domain.Run, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 200 {
		return domain.Run{}, false, fmt.Errorf("%w: Idempotency-Key must contain 1-200 characters", domain.ErrInvalid)
	}
	if err := validateRunInput(textInput, jsonInput); err != nil {
		return domain.Run{}, false, err
	}
	requestBody, err := marshal(map[string]any{"trigger": trigger, "text": textInput, "json": jsonInput})
	if err != nil {
		return domain.Run{}, false, err
	}
	digestBytes := sha256.Sum256(requestBody)
	digest := hex.EncodeToString(digestBytes[:])
	scope, operation := "workflow:"+workflowID, "create-run"
	var created runRecord
	replayed := false
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Exec(`INSERT INTO idempotency_records (owner_scope, operation, idempotency_key, request_digest) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING`, scope, operation, key, digest)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 0 {
			var record struct {
				RequestDigest string `gorm:"column:request_digest"`
				ResponseBody  []byte `gorm:"column:response_body"`
			}
			if err := tx.Raw(`SELECT request_digest, response_body FROM idempotency_records WHERE owner_scope = ? AND operation = ? AND idempotency_key = ? FOR UPDATE`, scope, operation, key).Scan(&record).Error; err != nil {
				return err
			}
			if record.RequestDigest != digest {
				return domain.ErrConflict
			}
			var response struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(record.ResponseBody, &response); err != nil || response.RunID == "" {
				return domain.ErrConflict
			}
			if err := tx.Where("owner_user_id = ? AND workflow_id = ? AND id = ?", ownerID, workflowID, response.RunID).Take(&created).Error; err != nil {
				return mapNotFound(err)
			}
			replayed = true
			return nil
		}
		created, err = createRunOnTx(tx, ownerID, workflowID, trigger, textInput, jsonInput)
		if err != nil {
			return err
		}
		response, _ := marshal(map[string]string{"run_id": created.ID})
		return tx.Exec(`UPDATE idempotency_records SET response_status = 202, response_body = ? WHERE owner_scope = ? AND operation = ? AND idempotency_key = ?`, response, scope, operation, key).Error
	})
	if err != nil {
		return domain.Run{}, false, fmt.Errorf("create idempotent Workflow Run: %w", err)
	}
	return runDomain(created), replayed, nil
}

func validateRunInput(textInput *string, jsonInput map[string]any) error {
	if textInput != nil && len(*textInput) > 100_000 {
		return fmt.Errorf("%w: Run text input exceeds 100000 bytes", domain.ErrInvalid)
	}
	encoded, err := json.Marshal(jsonInput)
	if err != nil {
		return fmt.Errorf("%w: encode Run JSON input: %v", domain.ErrInvalid, err)
	}
	if len(encoded) > 1_048_576 {
		return fmt.Errorf("%w: Run JSON input exceeds 1 MiB", domain.ErrInvalid)
	}
	return nil
}

func createRunOnTx(tx *gorm.DB, ownerID, workflowID, trigger string, textInput *string, jsonInput map[string]any) (runRecord, error) {
	var workflow workflowRecord
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).Take(&workflow).Error; err != nil {
		return runRecord{}, mapNotFound(err)
	}
	input, err := marshal(map[string]any{"text": textInput, "json": jsonInput})
	if err != nil {
		return runRecord{}, err
	}
	executionSnapshot, err := loadExecutionSnapshot(tx, workflow)
	if err != nil {
		return runRecord{}, err
	}
	snapshot, err := marshal(executionSnapshot)
	if err != nil {
		return runRecord{}, err
	}
	id := uuid.NewString()
	created := runRecord{ID: id, ConversationID: id, TurnNumber: 1, OwnerID: ownerID, WorkflowID: &workflowID, WorkflowName: workflow.Name, Trigger: trigger, State: "queued", Input: input, WorkflowSnapshot: snapshot, ExpertStages: []byte("[]"), QueuedAt: time.Now().UTC(), Version: 1}
	return created, tx.Create(&created).Error
}

func (repository *Repository) ContinueRunConversation(ctx context.Context, ownerID, workflowID, runID, content string, attachments []domain.Attachment) (domain.Run, error) {
	content = strings.TrimSpace(content)
	if content == "" && len(attachments) == 0 || len(content) > 100_000 {
		return domain.Run{}, fmt.Errorf("%w: follow-up must contain text or an attachment", domain.ErrInvalid)
	}
	var created runRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var workflow workflowRecord
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("owner_user_id = ? AND id = ? AND deleted_at IS NULL", ownerID, workflowID).Take(&workflow).Error; err != nil {
			return mapNotFound(err)
		}
		var root runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND workflow_id = ? AND id = ? AND conversation_id = id", ownerID, workflowID, runID).Take(&root).Error; err != nil {
			return mapNotFound(err)
		}
		var active int64
		if err := tx.Model(&runRecord{}).Where("conversation_id = ? AND state IN ('queued','running')", root.ID).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return domain.ErrConflict
		}
		var lastTurn int
		if err := tx.Model(&runRecord{}).Select("COALESCE(MAX(turn_number), 0)").Where("conversation_id = ?", root.ID).Scan(&lastTurn).Error; err != nil {
			return err
		}
		input, err := marshal(map[string]any{"text": content, "json": nil, "attachments": attachments})
		if err != nil {
			return err
		}
		created = runRecord{ID: uuid.NewString(), ConversationID: root.ID, TurnNumber: lastTurn + 1, OwnerID: ownerID, WorkflowID: root.WorkflowID, WorkflowName: root.WorkflowName, Trigger: "manual", State: "queued", Input: input, WorkflowSnapshot: append([]byte(nil), root.WorkflowSnapshot...), ExpertStages: []byte("[]"), QueuedAt: time.Now().UTC(), Version: 1}
		return tx.Create(&created).Error
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("continue Run Conversation: %w", err)
	}
	return runDomain(created), nil
}

func loadExecutionSnapshot(tx *gorm.DB, workflow workflowRecord) (domain.ExecutionSnapshot, error) {
	var settings settingsRecord
	if err := tx.Where("user_id = ?", workflow.OwnerID).Take(&settings).Error; err != nil {
		return domain.ExecutionSnapshot{}, fmt.Errorf("load Personal Settings for Run: %w", err)
	}
	snapshot := domain.ExecutionSnapshot{
		SchemaVersion: 2, WorkflowName: workflow.Name, Goal: workflow.Goal,
		Personality: settings.Personality, PersonalityInstructions: settings.PersonalityInstructions,
		EnvironmentSecretCiphertext: workflow.EnvironmentSecret, GitSecretCiphertext: workflow.GitSecret,
		WorkspacePath: workflow.WorkspacePath,
	}
	runtime, err := domain.ParseRuntime(settings.DefaultRuntimeEngine)
	if err != nil {
		return domain.ExecutionSnapshot{}, err
	}
	defaults := map[string]string{}
	if err := json.Unmarshal(settings.RuntimeModelDefaults, &defaults); err != nil {
		return domain.ExecutionSnapshot{}, fmt.Errorf("decode Runtime model defaults: %w", err)
	}
	providerModelID := defaults[string(runtime)]
	if len(workflow.Environment) > 0 {
		if err := json.Unmarshal(workflow.Environment, &snapshot.Environment); err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("decode Workflow environment snapshot: %w", err)
		}
	}
	if len(workflow.GitSource) > 0 && string(workflow.GitSource) != "null" {
		snapshot.GitSource = &domain.GitSource{}
		if err := json.Unmarshal(workflow.GitSource, snapshot.GitSource); err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("decode Workflow Git source snapshot: %w", err)
		}
	}
	if workflow.ExpertID == nil && workflow.ExpertTeamID == nil {
		stage, err := loadExecutionStage(tx, workflow.OwnerID, providerModelID, runtime, nil, nil, nil, 1)
		if err != nil {
			return domain.ExecutionSnapshot{}, err
		}
		return application.PlanExecution(snapshot, application.ExecutionSelection{Anonymous: &stage})
	}
	if workflow.ExpertID != nil {
		var expert expertRecord
		if err := tx.Where("owner_user_id = ? AND id = ?", workflow.OwnerID, *workflow.ExpertID).Take(&expert).Error; err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("load Expert for Run: %w", mapNotFound(err))
		}
		stage, err := loadExpertExecutionStage(tx, workflow.OwnerID, expert, providerModelID, runtime, 1)
		if err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("Expert %q: %w", expert.Name, err)
		}
		return application.PlanExecution(snapshot, application.ExecutionSelection{Expert: &stage})
	}
	var team expertTeamRecord
	if err := tx.Where("owner_user_id = ? AND id = ?", workflow.OwnerID, *workflow.ExpertTeamID).Take(&team).Error; err != nil {
		return domain.ExecutionSnapshot{}, fmt.Errorf("load Expert Team for Run: %w", mapNotFound(err))
	}
	var expertIDs []string
	var members []domain.ExpertTeamMemberInput
	if len(team.Members) > 0 && string(team.Members) != "null" {
		_ = json.Unmarshal(team.Members, &members)
	}
	if len(members) > 0 {
		for _, member := range members {
			expertIDs = append(expertIDs, member.ExpertID)
		}
	} else if err := json.Unmarshal(team.ExpertIDs, &expertIDs); err != nil {
		return domain.ExecutionSnapshot{}, err
	}
	if len(expertIDs) < 2 {
		return domain.ExecutionSnapshot{}, fmt.Errorf("%w: Expert Team requires at least two Experts", domain.ErrInvalid)
	}
	for index, expertID := range expertIDs {
		var expert expertRecord
		if err := tx.Where("owner_user_id = ? AND id = ?", workflow.OwnerID, expertID).Take(&expert).Error; err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("%w: Expert Team member is unavailable", domain.ErrInvalid)
		}
		stage, err := loadExpertExecutionStage(tx, workflow.OwnerID, expert, providerModelID, runtime, index+1)
		if err != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("Expert %q: %w", expert.Name, err)
		}
		if len(members) > 0 {
			stage.TeamMemberID = members[index].ID
			stage.TeamMemberName = members[index].Name
			stage.TeamMemberLabels = append([]string(nil), members[index].Labels...)
		}
		snapshot.Stages = append(snapshot.Stages, stage)
	}
	teamStages := snapshot.Stages
	snapshot.Stages = nil
	return application.PlanExecution(snapshot, application.ExecutionSelection{Team: teamStages})
}

func loadExpertExecutionStage(tx *gorm.DB, ownerID string, expert expertRecord, providerModelID string, runtime domain.RuntimeEngine, position int) (domain.ExecutionStageSnapshot, error) {
	member, err := loadExpertMemberSnapshot(tx, ownerID, expert, position)
	if err != nil {
		return domain.ExecutionStageSnapshot{}, err
	}
	expertSnapshot := member.ExpertSnapshot
	return loadExecutionStage(tx, ownerID, providerModelID, runtime, &expertSnapshot, member.MCPServers, member.Skills, position)
}

func loadExecutionStage(tx *gorm.DB, ownerID, providerModelID string, runtime domain.RuntimeEngine, expert *domain.ExpertSnapshot, servers []domain.MCPServerSnapshot, skills []domain.SkillSnapshot, position int) (domain.ExecutionStageSnapshot, error) {
	if providerModelID == "" {
		return domain.ExecutionStageSnapshot{}, fmt.Errorf("%w: choose a default Provider Model for %s", domain.ErrInvalid, runtime)
	}
	var model providerModelRecord
	if err := tx.Where("id = ? AND available", providerModelID).Take(&model).Error; err != nil {
		return domain.ExecutionStageSnapshot{}, fmt.Errorf("%w: selected Provider Model is unavailable", domain.ErrInvalid)
	}
	var connection modelProviderConnectionRecord
	if err := tx.Where("id = ?", model.ConnectionID).Take(&connection).Error; err != nil {
		return domain.ExecutionStageSnapshot{}, mapNotFound(err)
	}
	var protocols []string
	if err := json.Unmarshal(connection.Protocols, &protocols); err != nil {
		return domain.ExecutionStageSnapshot{}, err
	}
	parsed, err := providerModelDomain(model)
	if err != nil {
		return domain.ExecutionStageSnapshot{}, err
	}
	compatibility := "unverified"
	for _, item := range parsed.Compatibility {
		if item.RuntimeEngine == runtime {
			compatibility = item.Status
			break
		}
	}
	if compatibility == "incompatible" {
		return domain.ExecutionStageSnapshot{}, fmt.Errorf("%w: selected Provider Model is incompatible with %s", domain.ErrInvalid, runtime)
	}
	provider := domain.ProviderModelSnapshot{ID: model.ID, ConnectionID: connection.ID, ConnectionVersion: connection.Version, ConnectionName: connection.Name, ProviderType: connection.ProviderType, ModelID: model.ModelID, Name: model.DisplayName, Endpoint: connection.Endpoint, Protocols: protocols, Compatibility: compatibility, CredentialOwnerID: connection.CredentialOwnerID}
	protocol, err := domain.ModelProtocolForRuntime(runtime, protocols)
	if err != nil {
		return domain.ExecutionStageSnapshot{}, err
	}
	rate, err := loadCreditRateSnapshot(tx, connection.ProviderType, protocol, model.ModelID)
	if err != nil {
		return domain.ExecutionStageSnapshot{}, err
	}
	return domain.ExecutionStageSnapshot{Position: position, Expert: expert, RuntimeEngine: runtime, ProviderModel: provider, ModelProtocol: protocol, CreditRate: &rate, MCPServers: servers, Skills: skills}, nil
}

func loadCreditRateSnapshot(tx *gorm.DB, providerType, protocol, modelID string) (domain.CreditRateSnapshot, error) {
	var row struct {
		RevisionID             string `gorm:"column:id"`
		InputMultiplierMicros  int64  `gorm:"column:input_multiplier_micros"`
		OutputMultiplierMicros int64  `gorm:"column:output_multiplier_micros"`
		FallbackHundredths     int64  `gorm:"column:fallback_hundredths"`
	}
	query := tx.Table("model_credit_rate_revisions").Where("provider_type = ? AND api_protocol = ? AND provider_model_id = ? AND superseded_at IS NULL", providerType, protocol, modelID).Take(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		query = tx.Table("model_credit_rate_revisions").Where("provider_type IS NULL AND superseded_at IS NULL").Take(&row)
	}
	if query.Error != nil {
		return domain.CreditRateSnapshot{}, fmt.Errorf("resolve Model Credit Rate snapshot: %w", query.Error)
	}
	return domain.CreditRateSnapshot{RevisionID: row.RevisionID, InputMultiplierMicros: row.InputMultiplierMicros, OutputMultiplierMicros: row.OutputMultiplierMicros, FallbackHundredths: row.FallbackHundredths}, nil
}

func loadExpertMemberSnapshot(tx *gorm.DB, ownerID string, expert expertRecord, position int) (domain.ExpertMemberSnapshot, error) {
	structured := strings.TrimSpace(expert.Introduction) != "" && strings.TrimSpace(expert.CoreCapability) != "" && strings.TrimSpace(expert.OperatingProcedure) != "" && strings.TrimSpace(expert.OutputStandard) != ""
	if !structured && strings.TrimSpace(expert.ExecutionInstruction) == "" {
		return domain.ExpertMemberSnapshot{}, fmt.Errorf("%w: Expert guidance is incomplete", domain.ErrInvalid)
	}
	var tags, mcpIDs, skillIDs []string
	if err := json.Unmarshal(expert.ExpertiseTags, &tags); err != nil {
		return domain.ExpertMemberSnapshot{}, err
	}
	member := domain.ExpertMemberSnapshot{ExpertSnapshot: domain.ExpertSnapshot{ID: expert.ID, Name: expert.Name, Icon: expert.Icon, IconBackground: expert.IconBackground, Introduction: expert.Introduction, CoreCapability: expert.CoreCapability, OperatingProcedure: expert.OperatingProcedure, OutputStandard: expert.OutputStandard, Cautions: expert.Cautions, CapabilityIntroduction: expert.CapabilityIntroduction, ExecutionInstruction: expert.ExecutionInstruction, ExpertiseTags: tags, Version: expert.Version}, Position: position}
	if err := json.Unmarshal(expert.MCPServerIDs, &mcpIDs); err != nil {
		return domain.ExpertMemberSnapshot{}, err
	}
	if err := json.Unmarshal(expert.SkillIDs, &skillIDs); err != nil {
		return domain.ExpertMemberSnapshot{}, err
	}
	for _, id := range mcpIDs {
		var row mcpRecord
		if err := tx.Where("owner_user_id = ? AND id = ? AND tested_at IS NOT NULL AND test_error IS NULL", ownerID, id).Take(&row).Error; err != nil {
			return domain.ExpertMemberSnapshot{}, fmt.Errorf("%w: Expert MCP Server must pass its isolated test", domain.ErrInvalid)
		}
		member.MCPServers = append(member.MCPServers, domain.MCPServerSnapshot{ID: row.ID, Name: row.Name, Transport: row.Transport, Configuration: json.RawMessage(row.Configuration), SecretCiphertext: row.SecretCiphertext})
	}
	for _, id := range skillIDs {
		var row skillRecord
		if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, id).Take(&row).Error; err != nil {
			return domain.ExpertMemberSnapshot{}, fmt.Errorf("%w: Expert Skill is unavailable", domain.ErrInvalid)
		}
		member.Skills = append(member.Skills, domain.SkillSnapshot{ID: row.ID, Name: row.Name, ObjectKey: row.ObjectKey, SHA256: row.SHA256})
	}
	return member, nil
}

func (repository *Repository) ListRuns(ctx context.Context, ownerID, workflowID string) ([]domain.Run, error) {
	if _, err := repository.GetWorkflow(ctx, ownerID, workflowID, true); err != nil {
		return nil, err
	}
	var rows []runRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ?", ownerID, workflowID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Workflow Runs: %w", err)
	}
	summaries := summarizeRunConversations(rows)
	items := make([]domain.Run, 0, len(summaries))
	for _, row := range summaries {
		items = append(items, runDomain(row))
	}
	return items, nil
}

func summarizeRunConversations(rows []runRecord) []runRecord {
	roots := make(map[string]runRecord)
	latest := make(map[string]runRecord)
	for _, row := range rows {
		if row.ID == row.ConversationID {
			roots[row.ConversationID] = row
		}
		current, ok := latest[row.ConversationID]
		if !ok || row.TurnNumber > current.TurnNumber || row.TurnNumber == current.TurnNumber && row.QueuedAt.After(current.QueuedAt) {
			latest[row.ConversationID] = row
		}
	}

	summaries := make([]runRecord, 0, len(roots))
	for conversationID, root := range roots {
		turn := latest[conversationID]
		root.State = turn.State
		root.TurnNumber = turn.TurnNumber
		root.QueuedAt = turn.QueuedAt
		root.StartedAt = turn.StartedAt
		root.EndedAt = turn.EndedAt
		summaries = append(summaries, root)
	}
	sort.Slice(summaries, func(left, right int) bool {
		if summaries[left].QueuedAt.Equal(summaries[right].QueuedAt) {
			return summaries[left].ID > summaries[right].ID
		}
		return summaries[left].QueuedAt.After(summaries[right].QueuedAt)
	})
	return summaries
}

func (repository *Repository) ListRunTurns(ctx context.Context, ownerID, workflowID, runID string) ([]domain.Run, error) {
	var root runRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND id = ? AND conversation_id = id", ownerID, workflowID, runID).Take(&root).Error; err != nil {
		return nil, mapNotFound(err)
	}
	var rows []runRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND conversation_id = ?", ownerID, workflowID, root.ID).Order("turn_number, queued_at, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Run Conversation turns: %w", err)
	}
	items := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		items = append(items, runDomain(row))
	}
	return items, nil
}

func (repository *Repository) GetRun(ctx context.Context, ownerID, workflowID, runID string) (domain.Run, error) {
	var row runRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND id = ?", ownerID, workflowID, runID).Take(&row).Error; err != nil {
		return domain.Run{}, mapNotFound(err)
	}
	return runDomain(row), nil
}

func (repository *Repository) ListRunEvents(ctx context.Context, ownerID, workflowID, runID string, after int64, limit int) ([]domain.RunEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var rows []struct {
		Sequence   int64     `gorm:"column:sequence"`
		Type       string    `gorm:"column:event_type"`
		Payload    []byte    `gorm:"column:payload"`
		OccurredAt time.Time `gorm:"column:occurred_at"`
	}
	err := repository.db.WithContext(ctx).Table("run_events event").
		Joins("JOIN runs run ON run.id = event.run_id").
		Where("run.owner_user_id = ? AND run.workflow_id = ? AND run.id = ? AND event.sequence > ?", ownerID, workflowID, runID, after).
		Order("event.sequence").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list Run Events: %w", err)
	}
	items := make([]domain.RunEvent, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.RunEvent{Sequence: row.Sequence, Type: row.Type, Payload: append([]byte(nil), row.Payload...), OccurredAt: row.OccurredAt})
	}
	return items, nil
}

func (repository *Repository) CancelRun(ctx context.Context, ownerID, workflowID, runID string) (domain.Run, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ? AND workflow_id = ? AND id = ?", ownerID, workflowID, runID).Take(&run).Error; err != nil {
			return mapNotFound(err)
		}
		if run.State != "queued" && run.State != "running" {
			return domain.ErrConflict
		}
		now := time.Now().UTC()
		updates := map[string]any{"cancel_requested_at": now, "version": gorm.Expr("version + 1")}
		if run.State == "queued" {
			updates["state"] = "cancelled"
			updates["ended_at"] = now
		}
		if err := tx.Model(&runRecord{}).Where("id = ?", run.ID).Updates(updates).Error; err != nil {
			return err
		}
		if run.State == "queued" {
			return appendRunEvents(tx, run.ID, nil, "run.cancelled", now)
		}
		return nil
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("cancel Run: %w", err)
	}
	return repository.GetRun(ctx, ownerID, workflowID, runID)
}

func (repository *Repository) Rerun(ctx context.Context, ownerID, workflowID, runID string) (domain.Run, error) {
	var source runRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND id = ? AND state IN ('succeeded','failed','cancelled')", ownerID, workflowID, runID).Take(&source).Error; err != nil {
		return domain.Run{}, mapNotFound(err)
	}
	created := runRecord{
		ID: uuid.NewString(), OwnerID: ownerID, WorkflowID: source.WorkflowID, WorkflowName: source.WorkflowName,
		Trigger: "manual", State: "queued", Input: append([]byte(nil), source.Input...),
		WorkflowSnapshot: append([]byte(nil), source.WorkflowSnapshot...), ExpertStages: []byte("[]"), QueuedAt: time.Now().UTC(), Version: 1,
	}
	created.ConversationID = created.ID
	created.TurnNumber = 1
	if err := repository.db.WithContext(ctx).Create(&created).Error; err != nil {
		return domain.Run{}, fmt.Errorf("rerun Workflow: %w", err)
	}
	return runDomain(created), nil
}

func (repository *Repository) ListArtifacts(ctx context.Context, ownerID, workflowID string) ([]domain.Artifact, error) {
	if _, err := repository.GetWorkflow(ctx, ownerID, workflowID, true); err != nil {
		return nil, err
	}
	var rows []artifactRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND kind = 'file'", ownerID, workflowID).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Artifacts: %w", err)
	}
	items := make([]domain.Artifact, 0, len(rows))
	for _, row := range rows {
		item := domain.Artifact{ID: row.ID, RunID: row.RunID, Kind: row.Kind, Name: row.Name, Path: row.Path, Size: row.Size, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt}
		if row.ObjectKey != nil {
			item.ObjectKey = *row.ObjectKey
		}
		if row.SHA256 != nil {
			item.SHA256 = *row.SHA256
		}
		if len(row.TextResult) > 0 {
			_ = json.Unmarshal(row.TextResult, &item.TextPreview)
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) GetArtifact(ctx context.Context, ownerID, workflowID, artifactID string) (domain.Artifact, error) {
	var row artifactRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND workflow_id = ? AND id = ? AND kind = 'file'", ownerID, workflowID, artifactID).Take(&row).Error; err != nil {
		return domain.Artifact{}, mapNotFound(err)
	}
	item := domain.Artifact{ID: row.ID, RunID: row.RunID, Kind: row.Kind, Name: row.Name, Path: row.Path, Size: row.Size, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt}
	if row.ObjectKey != nil {
		item.ObjectKey = *row.ObjectKey
	}
	if row.SHA256 != nil {
		item.SHA256 = *row.SHA256
	}
	return item, nil
}

func workflowRecordForInput(id, ownerID, workspacePath string, input domain.WorkflowInput, secrets []byte) (workflowRecord, error) {
	environment, err := marshal(input.Environment)
	if err != nil {
		return workflowRecord{}, err
	}
	schedule, err := marshalNullable(input.Schedule)
	if err != nil {
		return workflowRecord{}, err
	}
	return workflowRecord{ID: id, OwnerID: ownerID, Name: strings.TrimSpace(input.Name), Goal: strings.TrimSpace(input.Goal), ExpertID: input.ExpertID, ExpertTeamID: input.ExpertTeamID, Environment: environment, EnvironmentSecret: secrets, Schedule: schedule, NextScheduledAt: nextScheduledAt(input.Schedule, time.Now().UTC()), WorkspacePath: workspacePath, Version: 1}, nil
}

func nextScheduledAt(schedule *domain.Schedule, after time.Time) *time.Time {
	if schedule == nil || !schedule.Enabled {
		return nil
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return nil
	}
	local := after.In(location)
	var next time.Time
	switch schedule.Frequency {
	case "hourly":
		next = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), int(schedule.Minute), 0, 0, location)
		if !next.After(local) {
			next = next.Add(time.Hour)
		}
	case "daily":
		next = time.Date(local.Year(), local.Month(), local.Day(), int(schedule.Hour), int(schedule.Minute), 0, 0, location)
		if !next.After(local) {
			next = next.AddDate(0, 0, 1)
		}
	case "weekly":
		days := (int(schedule.Weekday) - int(local.Weekday()) + 7) % 7
		next = time.Date(local.Year(), local.Month(), local.Day()+days, int(schedule.Hour), int(schedule.Minute), 0, 0, location)
		if !next.After(local) {
			next = next.AddDate(0, 0, 7)
		}
	default:
		return nil
	}
	utc := next.UTC()
	return &utc
}

func marshalNullable(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return marshal(value)
}

func workflowDomain(row workflowRecord) (domain.Workflow, error) {
	item := domain.Workflow{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Goal: row.Goal, ExpertID: row.ExpertID, ExpertTeamID: row.ExpertTeamID, ProviderModelID: row.ProviderModelID, APICredentialConfigured: row.APIKey != nil, WorkspacePath: row.WorkspacePath, DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
	if row.RuntimeEngine != nil {
		runtime, err := domain.ParseRuntime(*row.RuntimeEngine)
		if err != nil {
			return domain.Workflow{}, err
		}
		item.RuntimeEngine = &runtime
	}
	if len(row.Environment) > 0 {
		if err := json.Unmarshal(row.Environment, &item.Environment); err != nil {
			return domain.Workflow{}, fmt.Errorf("decode Workflow environment: %w", err)
		}
	}
	if len(row.Schedule) > 0 && string(row.Schedule) != "null" {
		item.Schedule = &domain.Schedule{}
		if err := json.Unmarshal(row.Schedule, item.Schedule); err != nil {
			return domain.Workflow{}, fmt.Errorf("decode Workflow schedule: %w", err)
		}
	}
	if len(row.GitSource) > 0 && string(row.GitSource) != "null" {
		item.GitSource = &domain.GitSource{}
		if err := json.Unmarshal(row.GitSource, item.GitSource); err != nil {
			return domain.Workflow{}, fmt.Errorf("decode Workflow Git source: %w", err)
		}
	}
	return item, nil
}

func runDomain(row runRecord) domain.Run {
	item := domain.Run{ID: row.ID, ConversationID: row.ConversationID, TurnNumber: row.TurnNumber, OwnerID: row.OwnerID, WorkflowName: row.WorkflowName, Trigger: row.Trigger, State: row.State, QueuedAt: row.QueuedAt, StartedAt: row.StartedAt, EndedAt: row.EndedAt}
	if row.WorkflowID != nil {
		item.WorkflowID = *row.WorkflowID
	}
	var input struct {
		Text        *string             `json:"text"`
		JSON        map[string]any      `json:"json"`
		Attachments []domain.Attachment `json:"attachments"`
	}
	_ = json.Unmarshal(row.Input, &input)
	item.TextInput, item.JSONInput, item.Attachments = input.Text, input.JSON, input.Attachments
	if len(row.ExpertStages) > 0 && string(row.ExpertStages) != "null" {
		_ = json.Unmarshal(row.ExpertStages, &item.ExpertStages)
	}
	if len(row.CreditConsumption) > 0 && string(row.CreditConsumption) != "null" {
		item.CreditConsumption = &domain.CreditConsumption{}
		if err := json.Unmarshal(row.CreditConsumption, item.CreditConsumption); err != nil {
			item.CreditConsumption = nil
		}
	}
	if row.TerminalError != nil {
		item.Error = *row.TerminalError
	}
	if len(row.FinalResult) > 0 {
		var result struct {
			Text *string        `json:"text"`
			JSON map[string]any `json:"json"`
		}
		_ = json.Unmarshal(row.FinalResult, &result)
		item.FinalText, item.FinalJSON = result.Text, result.JSON
	}
	var snapshot domain.ExecutionSnapshot
	if json.Unmarshal(row.WorkflowSnapshot, &snapshot) == nil {
		projection := map[string]any{
			"workflow_name": snapshot.WorkflowName, "goal": snapshot.Goal, "runtime_engine": string(snapshot.RuntimeEngine),
			"provider_model": map[string]any{"id": snapshot.ProviderModel.ID, "connection_id": snapshot.ProviderModel.ConnectionID, "connection_name": snapshot.ProviderModel.ConnectionName, "provider_type": snapshot.ProviderModel.ProviderType, "name": snapshot.ProviderModel.Name, "model_id": snapshot.ProviderModel.ModelID, "endpoint": snapshot.ProviderModel.Endpoint, "protocols": snapshot.ProviderModel.Protocols},
			"personality":    snapshot.Personality, "environment": snapshot.Environment, "workspace_path": snapshot.WorkspacePath,
		}
		if snapshot.Expert != nil {
			projection["expert"] = map[string]any{"id": snapshot.Expert.ID, "name": snapshot.Expert.Name}
		}
		if snapshot.ExpertTeam != nil {
			members := make([]map[string]any, 0, len(snapshot.ExpertTeam.Members))
			for _, member := range snapshot.ExpertTeam.Members {
				members = append(members, map[string]any{"id": member.ID, "name": member.Name, "position": member.Position})
			}
			projection["expert_team"] = map[string]any{"id": snapshot.ExpertTeam.ID, "name": snapshot.ExpertTeam.Name, "members": members}
		}
		mcp := make([]map[string]any, 0, len(snapshot.MCPServers))
		for _, server := range snapshot.MCPServers {
			mcp = append(mcp, map[string]any{"id": server.ID, "name": server.Name, "transport": server.Transport})
		}
		projection["mcp_servers"] = mcp
		skills := make([]map[string]any, 0, len(snapshot.Skills))
		for _, skill := range snapshot.Skills {
			skills = append(skills, map[string]any{"id": skill.ID, "name": skill.Name, "sha256": skill.SHA256})
		}
		projection["skills"] = skills
		if stages, err := snapshot.OrderedStages(); err == nil {
			stageProjection := make([]map[string]any, 0, len(stages))
			for _, stage := range stages {
				value := map[string]any{"position": stage.Position, "runtime_engine": string(stage.RuntimeEngine), "provider_model": map[string]any{"id": stage.ProviderModel.ID, "connection_id": stage.ProviderModel.ConnectionID, "connection_name": stage.ProviderModel.ConnectionName, "provider_type": stage.ProviderModel.ProviderType, "name": stage.ProviderModel.Name, "model_id": stage.ProviderModel.ModelID, "endpoint": stage.ProviderModel.Endpoint, "protocols": stage.ProviderModel.Protocols, "compatibility": stage.ProviderModel.Compatibility}}
				if stage.Expert != nil {
					value["expert"] = map[string]any{"id": stage.Expert.ID, "name": stage.Expert.Name, "version": stage.Expert.Version}
				}
				stageProjection = append(stageProjection, value)
			}
			projection["execution_stages"] = stageProjection
		}
		item.WorkflowSnapshot = projection
	}
	return item
}
