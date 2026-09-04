package agentruntime

import (
	"errors"
	"testing"
)

func TestDescriptorSupportsDeclaredCapabilities(t *testing.T) {
	descriptor := Descriptor{
		Capabilities: map[Capability]bool{
			CapabilityStreaming: true,
		},
	}

	if !descriptor.Supports(CapabilityStreaming) {
		t.Fatal("expected streaming capability")
	}
	if descriptor.Supports(CapabilityNativeResume) {
		t.Fatal("did not expect native resume capability")
	}
}

func TestExecuteRequestValidatesAttachments(t *testing.T) {
	valid := ExecuteRequest{
		RunID: "run-1", WorkspacePath: "/workspace", Model: "model-1", EnvironmentRef: "environment-1",
		Attachments: []Attachment{{Path: "/workspace/.agent-platform-attachments/id/photo.png", ContentType: "image/png"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}

	tests := []struct {
		name        string
		attachments []Attachment
	}{
		{name: "relative path", attachments: []Attachment{{Path: "photo.png", ContentType: "image/png"}}},
		{name: "root path", attachments: []Attachment{{Path: "/", ContentType: "image/png"}}},
		{name: "outside attachment boundary", attachments: []Attachment{{Path: "/run/agent-credentials/photo.png", ContentType: "image/png"}}},
		{name: "missing content type", attachments: []Attachment{{Path: "/workspace/.agent-platform-attachments/id/photo.png"}}},
		{name: "too many", attachments: make([]Attachment, 11)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			request.Attachments = test.attachments
			var runtimeErr *Error
			if err := request.Validate(); !errors.As(err, &runtimeErr) || runtimeErr.Code != ErrorInvalidConfiguration {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
