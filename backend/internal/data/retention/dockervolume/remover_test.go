package dockervolume

import (
	"context"
	"strings"
	"testing"
)

type executorFunc func(context.Context, ...string) (string, error)

func (function executorFunc) Run(ctx context.Context, arguments ...string) (string, error) {
	return function(ctx, arguments...)
}

func TestRemoverOnlyDeletesManagedWorkspaceVolume(t *testing.T) {
	called := false
	remover, err := New(executorFunc(func(_ context.Context, arguments ...string) (string, error) {
		called = true
		if strings.Join(arguments, " ") != "volume rm --force agent-platform-session-6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
			t.Fatalf("arguments = %q", arguments)
		}
		return "", nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := remover.Remove(context.Background(), "agent-platform-session-6ba7b810-9dad-11d1-80b4-00c04fd430c8"); err != nil || !called {
		t.Fatalf("Remove() called=%v error=%v", called, err)
	}
	for _, volume := range []string{"", "postgres-data", "agent-platform-session-../../data"} {
		if err := remover.Remove(context.Background(), volume); err == nil {
			t.Fatalf("Remove(%q) succeeded", volume)
		}
	}
}
