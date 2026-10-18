package runtimeexecutor

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestRemoveReleasedMountpointWaitsForLocalBindRelease(t *testing.T) {
	for _, transient := range []error{syscall.EACCES, syscall.EPERM, syscall.EBUSY} {
		t.Run(transient.Error(), func(t *testing.T) {
			attempts := 0
			err := removeReleasedMountpoint(t.Context(), "local", func() error {
				attempts++
				if attempts < 3 {
					return &os.PathError{Op: "remove", Path: "attachments", Err: transient}
				}
				return nil
			})
			if err != nil || attempts != 3 {
				t.Fatalf("cleanup attempts=%d error=%v", attempts, err)
			}
		})
	}
}

func TestRemoveReleasedMountpointDoesNotMaskErrors(t *testing.T) {
	for _, profile := range []string{"", "production", "local"} {
		t.Run(profile, func(t *testing.T) {
			attempts := 0
			failure := error(syscall.EACCES)
			if profile == "local" {
				failure = syscall.ENOTEMPTY
			}
			err := removeReleasedMountpoint(t.Context(), profile, func() error { attempts++; return failure })
			if !errors.Is(err, failure) || attempts != 1 {
				t.Fatalf("cleanup attempts=%d error=%v", attempts, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := removeReleasedMountpoint(ctx, "local", func() error { return syscall.EACCES })
	if !errors.Is(err, context.Canceled) || !errors.Is(err, syscall.EACCES) {
		t.Fatalf("cancellation lost cause: %v", err)
	}
}

func TestRemoveReleasedMountpointRetryIsBounded(t *testing.T) {
	attempts := 0
	err := removeReleasedMountpoint(t.Context(), "local", func() error { attempts++; return syscall.EACCES })
	if !errors.Is(err, syscall.EACCES) || attempts < 2 || attempts > 22 {
		t.Fatalf("unbounded cleanup attempts=%d error=%v", attempts, err)
	}
}
