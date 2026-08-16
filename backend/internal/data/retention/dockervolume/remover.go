package dockervolume

import (
	"context"
	"fmt"
	"regexp"
)

var managedVolumePattern = regexp.MustCompile(`^agent-platform-session-[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Executor interface {
	Run(context.Context, ...string) (string, error)
}

type Remover struct{ executor Executor }

func New(executor Executor) (*Remover, error) {
	if executor == nil {
		return nil, fmt.Errorf("Docker CLI Executor is required")
	}
	return &Remover{executor: executor}, nil
}

func (remover *Remover) Remove(ctx context.Context, volume string) error {
	if !managedVolumePattern.MatchString(volume) {
		return fmt.Errorf("refusing to delete unmanaged Workspace Volume")
	}
	if _, err := remover.executor.Run(ctx, "volume", "rm", "--force", volume); err != nil {
		return fmt.Errorf("delete Workspace Volume: %w", err)
	}
	return nil
}
