package main

import (
	"context"
	"fmt"
	"os"

	"agent-platform/backend/internal/gitworkflow"
)

func main() {
	plan, err := gitworkflow.DecodePlan(os.Getenv("AGENT_PLATFORM_WORKFLOW_B64"))
	if err == nil {
		err = (gitworkflow.Workflow{Plan: plan, Workspace: "/workspace", CredentialRoot: "/run/agent-credentials", ApprovalGate: gitworkflow.PauseForApproval}).Execute(context.Background(), os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "runtime workflow failed:", err)
		os.Exit(1)
	}
}
