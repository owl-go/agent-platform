package gormrepo

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateRunInputBoundsTextAndJSON(t *testing.T) {
	tooLong := strings.Repeat("x", 100_001)
	if err := validateRunInput(&tooLong, nil); err == nil {
		t.Fatal("validateRunInput accepted text above 100000 bytes")
	}
	valid := strings.Repeat("x", 100_000)
	if err := validateRunInput(&valid, map[string]any{"ok": true}); err != nil {
		t.Fatalf("validateRunInput rejected valid input: %v", err)
	}
	if err := validateRunInput(nil, map[string]any{"payload": strings.Repeat("x", 1_048_576)}); err == nil {
		t.Fatal("validateRunInput accepted JSON above 1 MiB")
	}
}

func TestWorkflowRunInstructionIncludesPriorConversation(t *testing.T) {
	firstInput, _ := json.Marshal(map[string]any{"text": "先检查代码", "json": nil})
	firstResult, _ := json.Marshal(map[string]any{"text": "发现两个问题", "json": nil})
	current := "继续修复第一个问题"
	instruction := workflowRunInstruction("完成代码审查", []runRecord{{Input: firstInput, FinalResult: firstResult}}, &current, nil)

	for _, expected := range []string{"Workflow goal:\n完成代码审查", "user: 先检查代码", "assistant: 发现两个问题", "Current user message:\n继续修复第一个问题"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("instruction %q does not contain %q", instruction, expected)
		}
	}
}

func TestBoundedSummaryPreservesUTF8(t *testing.T) {
	value := strings.Repeat("你", 10_000)
	bounded := boundedSummary(value)
	if !utf8.ValidString(bounded) {
		t.Fatal("boundedSummary split a UTF-8 rune")
	}
	if len(bounded) > 20_000 {
		t.Fatalf("bounded summary is %d bytes", len(bounded))
	}
}
