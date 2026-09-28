package domain

import "time"

type HomeOverview struct {
	RecentTasks     []HomeTask
	CommonWorkflows []HomeWorkflow
	ActionItems     []HomeAction
}

type HomeTask struct {
	Kind      string
	ID        string
	ParentID  string
	Title     string
	State     string
	UpdatedAt time.Time
}

type HomeWorkflow struct {
	ID        string
	Name      string
	RunCount  int64
	UpdatedAt time.Time
}

type HomeAction struct {
	Kind          string
	ID            string
	ExecutionKind string
	ExecutionID   string
	ParentID      string
	Title         string
	State         string
	CreatedAt     time.Time
}
