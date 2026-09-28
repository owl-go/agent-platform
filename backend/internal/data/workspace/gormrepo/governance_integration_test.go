package gormrepo

import (
	"context"
	"errors"
	"testing"
	"time"

	accountdomain "agent-platform/backend/internal/biz/account/domain"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	accountrepo "agent-platform/backend/internal/data/account/gormrepo"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"

	"github.com/google/uuid"
)

func TestEnterpriseGovernanceKeepsDepartmentResourcesAndPrivateContentInSeparateBoundaries(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	accounts := accountrepo.New(db)
	workspace := New(db, nil)
	credits := creditsrepo.New(db)

	administratorID := uuid.NewString()
	publisherID := uuid.NewString()
	receiverID := uuid.NewString()
	memberID := uuid.NewString()
	outsiderID := uuid.NewString()
	insertUser := func(id, subject, username string, administrator, publisher bool) {
		t.Helper()
		if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator,bootstrap_administrator,resource_publisher)
			VALUES(?,?,?,?,?,?,?,?)`, id, subject, username, username+"@example.test", username, administrator, administrator, publisher).Error; err != nil {
			t.Fatal(err)
		}
	}
	insertUser(administratorID, "subject-admin", "admin", true, false)
	insertUser(publisherID, "subject-publisher", "publisher", false, true)
	insertUser(receiverID, "subject-receiver", "receiver", false, true)
	insertUser(memberID, "subject-member", "member", false, false)
	insertUser(outsiderID, "subject-outsider", "outsider", false, false)

	groups, err := accounts.ReplaceIdentityGroups(ctx, administratorID, []accountdomain.IdentityGroupSnapshot{{
		ExternalID: "keycloak-finance", Name: "Finance", Path: "/Company/Finance", Department: true,
		MemberSubjects: []string{"subject-publisher", "subject-receiver", "subject-member"},
	}}, time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC))
	if err != nil || len(groups) != 1 || groups[0].MemberCount != 3 {
		t.Fatalf("synchronize Department = %#v, %v", groups, err)
	}
	groupID := groups[0].ID

	member, err := accounts.SetRoles(ctx, administratorID, memberID, true, false, 1, "delegate regional administration")
	if err != nil || !member.Administrator {
		t.Fatalf("promote second Administrator = %#v, %v", member, err)
	}

	departmentBase, err := workspace.CreateKnowledgeBase(ctx, publisherID, false, workspacedomain.KnowledgeBaseInput{
		Name: "Finance policy", Visibility: workspacedomain.KnowledgePrivate, Scope: workspacedomain.KnowledgeScopeGroup, GroupID: &groupID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.GetKnowledgeBase(ctx, receiverID, departmentBase.ID, false, false); err != nil {
		t.Fatalf("Department member could not read shared Knowledge Base: %v", err)
	}
	updated, err := workspace.UpdateKnowledgeBase(ctx, receiverID, departmentBase.ID, false, workspacedomain.KnowledgeBaseInput{
		Name: "Finance policy 2026", Visibility: workspacedomain.KnowledgePrivate, Scope: workspacedomain.KnowledgeScopeGroup, GroupID: &groupID,
	}, departmentBase.Version)
	if err != nil || updated.Name != "Finance policy 2026" {
		t.Fatalf("Resource Publisher could not maintain Department Knowledge Base: %#v, %v", updated, err)
	}
	if _, err := workspace.GetKnowledgeBase(ctx, outsiderID, departmentBase.ID, false, false); !errors.Is(err, workspacedomain.ErrNotFound) {
		t.Fatalf("outsider Department access error = %v, want not found", err)
	}
	if _, err := workspace.GetKnowledgeBase(ctx, administratorID, departmentBase.ID, true, false); !errors.Is(err, workspacedomain.ErrNotFound) {
		t.Fatalf("non-member Administrator Department access error = %v, want not found", err)
	}

	privateBase, err := workspace.CreateKnowledgeBase(ctx, publisherID, false, workspacedomain.KnowledgeBaseInput{
		Name: "Private notes", Visibility: workspacedomain.KnowledgePrivate, Scope: workspacedomain.KnowledgeScopePrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.GetKnowledgeBase(ctx, administratorID, privateBase.ID, true, false); !errors.Is(err, workspacedomain.ErrNotFound) {
		t.Fatalf("Administrator private Knowledge Base access error = %v, want not found", err)
	}

	disabled, err := accounts.SetEnabled(ctx, administratorID, publisherID, false, 1, "employee offboarding")
	if err != nil || disabled.Enabled {
		t.Fatalf("disable departing publisher = %#v, %v", disabled, err)
	}
	groups, err = accounts.ReplaceIdentityGroups(ctx, administratorID, []accountdomain.IdentityGroupSnapshot{{
		ExternalID: "keycloak-finance", Name: "Finance", Path: "/Company/Finance", Department: true,
		MemberSubjects: []string{"subject-receiver", "subject-member"},
	}}, time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC))
	if err != nil || len(groups) != 1 || groups[0].MemberCount != 2 {
		t.Fatalf("synchronize offboarded Department membership = %#v, %v", groups, err)
	}
	count, err := workspace.TransferGroupResources(ctx, administratorID, groupID, publisherID, receiverID, "transfer Finance resource custody")
	if err != nil || count != 1 {
		t.Fatalf("transfer Department resources count = %d, %v", count, err)
	}
	var owners []struct {
		ID      string `gorm:"column:id"`
		OwnerID string `gorm:"column:owner_user_id"`
	}
	if err := db.Table("knowledge_bases").Select("id, owner_user_id").Where("id IN ?", []string{departmentBase.ID, privateBase.ID}).Order("id").Scan(&owners).Error; err != nil {
		t.Fatal(err)
	}
	ownerByID := map[string]string{}
	for _, owner := range owners {
		ownerByID[owner.ID] = owner.OwnerID
	}
	if ownerByID[departmentBase.ID] != receiverID || ownerByID[privateBase.ID] != publisherID {
		t.Fatalf("offboarding crossed resource boundary: %#v", ownerByID)
	}

	limit := int64(1_000)
	group, err := accounts.UpdateIdentityGroupBudget(ctx, administratorID, groupID, &limit, groups[0].Version, "cap Finance daily usage")
	if err != nil || group.DailyCreditLimitHundredths == nil || *group.DailyCreditLimitHundredths != limit {
		t.Fatalf("configure Department budget = %#v, %v", group, err)
	}
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	if _, err := credits.Balance(ctx, memberID, "UTC", now); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO credit_ledger(user_id,entry_type,amount_hundredths,daily_delta_hundredths,resulting_balance_hundredths,credit_day,source)
		VALUES(?,'consumption',-800,-800,59200,?::date,?)`, receiverID, now.Format(time.DateOnly), "test:department-consumption").Error; err != nil {
		t.Fatal(err)
	}
	rate, err := credits.ResolveRate(ctx, creditsdomain.ModelRateKey{ProviderType: "openai", Protocol: "openai_responses", ModelID: "governance-test"})
	if err != nil {
		t.Fatal(err)
	}
	rate.Fallback = 300
	_, err = credits.Admit(ctx, creditsdomain.Admission{UserID: memberID, ExecutionID: "group-budget", StagePosition: 1, Source: "session:group-budget", Timezone: "UTC", CreditDay: now.Format(time.DateOnly), StartedAt: now, Rate: rate})
	if !errors.Is(err, creditsdomain.ErrInsufficientCredits) {
		t.Fatalf("Department budget admission error = %v, want insufficient Credits", err)
	}
	groups, err = accounts.ReplaceIdentityGroups(ctx, administratorID, []accountdomain.IdentityGroupSnapshot{{
		ExternalID: "keycloak-finance", Name: "Finance", Path: "/Company/Finance", Department: false,
		MemberSubjects: []string{"subject-receiver", "subject-member"},
	}}, time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC))
	if err != nil || len(groups) != 1 || groups[0].DailyCreditLimitHundredths != nil {
		t.Fatalf("remove Department designation and budget = %#v, %v", groups, err)
	}
	if _, err := workspace.GetKnowledgeBase(ctx, receiverID, departmentBase.ID, false, false); !errors.Is(err, workspacedomain.ErrNotFound) {
		t.Fatalf("non-Department Group retained resource access: %v", err)
	}

	audit, err := accounts.ListGovernanceAuditEvents(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, event := range audit {
		actions[event.Action] = true
	}
	for _, action := range []string{"identity.groups.synced", "user.roles.updated", "user.enabled.updated", "identity_group.budget.updated", "group.resources.transferred"} {
		if !actions[action] {
			t.Errorf("governance audit omitted %q: %#v", action, actions)
		}
	}
}
