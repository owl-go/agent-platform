package agentruntime

import "testing"

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
