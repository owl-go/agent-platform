package domain

func (credit CreditStageConsumption) TaskPanelView() CreditStageConsumption {
	return CreditStageConsumption{StagePosition: credit.StagePosition, AmountHundredths: credit.AmountHundredths, Estimated: credit.Estimated, UsageReported: credit.UsageReported}
}

// Accounting detail remains internal to durable execution records. Coordination
// control responses are never member results or the official answer.
func (stage ExpertStage) TaskPanelView() ExpertStage {
	if stage.InvocationID == "" {
		return stage
	}
	stage.ProviderModelID, stage.ProviderModelName, stage.RuntimeEngine = "", "", ""
	if stage.Role == "lead" {
		stage.FinalText = ""
	}
	if stage.CreditConsumption != nil {
		credit := stage.CreditConsumption.TaskPanelView()
		stage.CreditConsumption = &credit
	}
	return stage
}
