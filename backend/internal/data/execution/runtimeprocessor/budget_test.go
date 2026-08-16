package runtimeprocessor

import (
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
)

func TestModelBudgetChecksEveryHardLimit(t *testing.T) {
	budget, err := parseModelBudget([]byte(`{"max_input_tokens":100,"max_output_tokens":50,"max_cost_amount":"1.25"}`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		usage agentruntime.Usage
		want  string
	}{
		{name: "within budget", usage: agentruntime.Usage{InputTokens: 100, OutputTokens: 50, CostMicros: 1_250_000}},
		{name: "input", usage: agentruntime.Usage{InputTokens: 101}, want: "input token limit exceeded"},
		{name: "output", usage: agentruntime.Usage{InputTokens: 100, OutputTokens: 51}, want: "output token limit exceeded"},
		{name: "cost", usage: agentruntime.Usage{InputTokens: 100, OutputTokens: 50, CostMicros: 1_250_001}, want: "model cost limit exceeded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if reason := budget.ExceededBy(test.usage); !strings.Contains(reason, test.want) {
				t.Fatalf("ExceededBy() = %q, want substring %q", reason, test.want)
			}
		})
	}
}

func TestParseModelBudgetFailsClosed(t *testing.T) {
	for _, value := range []string{
		`{}`,
		`{"max_input_tokens":100,"max_output_tokens":50,"max_cost_amount":"0"}`,
		`{"max_input_tokens":100,"max_output_tokens":50,"max_cost_amount":"1.00","unknown":true}`,
	} {
		if _, err := parseModelBudget([]byte(value)); err == nil {
			t.Fatalf("parseModelBudget(%s) succeeded", value)
		}
	}
}
