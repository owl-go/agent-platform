package runtimeexecutor

import (
	"context"
	"errors"
	"syscall"
	"time"

	"agent-platform/backend/internal/sandbox"
)

// Docker Desktop can return from stop before VirtioFS releases a nested
// read-only bind mount. Retry that specific local cleanup operation, without
// changing permissions or masking permanent errors. Production is unchanged.
func removeReleasedMountpoint(ctx context.Context, profile string, remove func() error) error {
	deadline := time.Now().Add(time.Second)
	for {
		err := remove()
		if err == nil || profile != sandbox.LocalProfile || time.Now().After(deadline) ||
			(!errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.EBUSY)) {
			return err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
}
