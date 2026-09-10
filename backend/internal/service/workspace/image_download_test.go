package workspace

import "testing"

func TestImageGenerationPath(t *testing.T) {
	tests := []struct {
		path, action, record, value string
		valid                       bool
	}{
		{"/api/v1/ai-creation/image-generations/record-1/images/2", "images", "record-1", "2", true},
		{"/api/v1/ai-creation/image-generations/record-1/download", "download", "record-1", "", true},
		{"/api/v1/ai-creation/image-generations/record-1/events", "events", "record-1", "", true},
		{"/api/v1/ai-creation/image-generations/record-1/images", "images", "", "", false},
	}
	for _, test := range tests {
		record, value, valid := imageGenerationPath(test.path, test.action)
		if record != test.record || value != test.value || valid != test.valid {
			t.Errorf("imageGenerationPath(%q, %q) = %q, %q, %v", test.path, test.action, record, value, valid)
		}
	}
}
