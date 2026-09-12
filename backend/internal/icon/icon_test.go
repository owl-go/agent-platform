package icon

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "built in", value: "sparkles", valid: true},
		{name: "png data url", value: "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png")), valid: true},
		{name: "unsupported mime", value: "data:image/svg+xml;base64,abc", valid: false},
		{name: "javascript url", value: "javascript:alert(1)", valid: false},
		{name: "oversized image", value: "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, MaxDataURLBytes+1)), valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Validate(test.value) == nil; got != test.valid {
				t.Fatalf("Validate() = %v, want %v", got, test.valid)
			}
		})
	}
	if IsDataURL(strings.TrimSpace("data:image/png;base64,abc")) != true {
		t.Fatal("IsDataURL should recognize image data URLs")
	}
}
