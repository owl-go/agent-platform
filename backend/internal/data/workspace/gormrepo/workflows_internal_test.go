package gormrepo

import (
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
