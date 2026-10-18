package runtimeexecutor

import "testing"

func TestUnifiedInstancesNeverShareWarmContainerNames(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	runtime := executor.config.Worker.Runtimes[string(job.Snapshot.RuntimeEngine)]
	legacy, _, err := executor.warmSlot(job, runtime)
	if err != nil {
		t.Fatal(err)
	}
	executor.config.Worker.RuntimeNamespace = "volume-a"
	first, _, err := executor.warmSlot(job, runtime)
	if err != nil {
		t.Fatal(err)
	}
	executor.config.Worker.RuntimeNamespace = "volume-b"
	second, _, err := executor.warmSlot(job, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first == legacy || second == legacy {
		t.Fatal("different control-container data volumes shared a warm Runtime")
	}
}
