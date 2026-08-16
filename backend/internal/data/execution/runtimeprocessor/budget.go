package runtimeprocessor

import (
	"fmt"
	"math/big"

	"agent-platform/backend/internal/agentruntime"
)

type ModelBudget struct {
	MaxInputTokens  int64
	MaxOutputTokens int64
	maxCost         *big.Rat
}

func parseModelBudget(value []byte) (ModelBudget, error) {
	var persisted struct {
		MaxInputTokens  int64  `json:"max_input_tokens"`
		MaxOutputTokens int64  `json:"max_output_tokens"`
		MaxCostAmount   string `json:"max_cost_amount"`
	}
	if err := decodeStrict(value, &persisted); err != nil {
		return ModelBudget{}, err
	}
	maxCost, ok := new(big.Rat).SetString(persisted.MaxCostAmount)
	if persisted.MaxInputTokens <= 0 || persisted.MaxOutputTokens <= 0 || !ok || maxCost.Sign() <= 0 {
		return ModelBudget{}, fmt.Errorf("limits must contain positive input tokens, output tokens, and cost")
	}
	return ModelBudget{
		MaxInputTokens: persisted.MaxInputTokens, MaxOutputTokens: persisted.MaxOutputTokens, maxCost: maxCost,
	}, nil
}

func (budget ModelBudget) ExceededBy(usage agentruntime.Usage) string {
	if usage.InputTokens > budget.MaxInputTokens {
		return fmt.Sprintf("input token limit exceeded: used %d, limit %d", usage.InputTokens, budget.MaxInputTokens)
	}
	if usage.OutputTokens > budget.MaxOutputTokens {
		return fmt.Sprintf("output token limit exceeded: used %d, limit %d", usage.OutputTokens, budget.MaxOutputTokens)
	}
	cost := new(big.Rat).SetFrac(big.NewInt(usage.CostMicros), big.NewInt(1_000_000))
	if cost.Cmp(budget.maxCost) > 0 {
		return fmt.Sprintf("model cost limit exceeded: used %s, limit %s", dollars(usage.CostMicros), budget.maxCost.FloatString(8))
	}
	return ""
}
