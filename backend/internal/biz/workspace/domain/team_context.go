package domain

type TeamRosterMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type TeamExecutionContext struct {
	ID                     string             `json:"id"`
	Name                   string             `json:"name"`
	LeadMemberID           string             `json:"lead_member_id"`
	Members                []TeamRosterMember `json:"members"`
	MaxModelCalls          int                `json:"max_model_calls"`
	MaxParallel            int                `json:"max_parallel"`
	ActiveTimeoutSeconds   int                `json:"active_timeout_seconds"`
	CreditBudgetHundredths int64              `json:"credit_budget_hundredths"`
}

func PublicTeamContext(coordination *TeamCoordinationSnapshot, profile *ExpertTeamProfileSnapshot, stages []ExecutionStageSnapshot) *TeamExecutionContext {
	if coordination == nil {
		return nil
	}
	config := coordination.Limits()
	result := &TeamExecutionContext{LeadMemberID: config.LeadMemberID, MaxModelCalls: config.MaxInvocations, MaxParallel: config.MaxParallel, ActiveTimeoutSeconds: config.ActiveTimeoutSeconds, CreditBudgetHundredths: config.CreditBudgetHundredths}
	if profile != nil {
		result.ID = profile.ID
		result.Name = profile.Name
	}
	for _, stage := range stages {
		result.Members = append(result.Members, TeamRosterMember{ID: stage.TeamMemberID, Name: stage.TeamMemberName})
	}
	return result
}
