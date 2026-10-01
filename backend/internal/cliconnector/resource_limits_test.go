package cliconnector

import "testing"

func TestExecutionLimitsPreservesMostRestrictiveSelectedPolicy(t *testing.T) {
	limits := ExecutionLimits(Definition{CPUMillis: 500, MemoryMiB: 512, ChildProcesses: 64}, Definition{CPUMillis: 800, MemoryMiB: 256, ChildProcesses: 16})
	if limits.CPUs != 0.5 || limits.MemoryBytes != 256<<20 || limits.PIDs != 16 || limits.TempBytes != 256<<20 {
		t.Fatalf("selected resource policy widened: %#v", limits)
	}
	defaults := ExecutionLimits(Definition{CPUMillis: 16000, MemoryMiB: 65536, ChildProcesses: 1024})
	if defaults.CPUs != 1 || defaults.MemoryBytes != 1<<30 || defaults.PIDs != 128 {
		t.Fatalf("platform resource cap widened: %#v", defaults)
	}
}
