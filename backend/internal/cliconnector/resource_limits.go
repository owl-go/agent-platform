package cliconnector

import "agent-platform/backend/internal/sandbox"

// ExecutionLimits applies the same reviewed resource caps to command execution
// and startup Conformance. Omitted limits retain the platform defaults.
func ExecutionLimits(definitions ...Definition) sandbox.Limits {
	limits := sandbox.Limits{CPUs: 1, MemoryBytes: 1 << 30, PIDs: 128, TempBytes: 256 << 20}
	for _, definition := range definitions {
		if definition.CPUMillis > 0 && float64(definition.CPUMillis)/1000 < limits.CPUs {
			limits.CPUs = float64(definition.CPUMillis) / 1000
		}
		if definition.MemoryMiB > 0 && int64(definition.MemoryMiB)*1024*1024 < limits.MemoryBytes {
			limits.MemoryBytes = int64(definition.MemoryMiB) * 1024 * 1024
		}
		if definition.ChildProcesses > 0 && int64(definition.ChildProcesses) < limits.PIDs {
			limits.PIDs = int64(definition.ChildProcesses)
		}
	}
	return limits
}
