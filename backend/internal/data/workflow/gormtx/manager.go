package gormtx

import (
	"context"
	"fmt"

	"agent-platform/backend/internal/biz/workflow"
	approvalgorm "agent-platform/backend/internal/data/approval/gormrepo"
	collaborationgorm "agent-platform/backend/internal/data/collaboration/gormrepo"
	executiongorm "agent-platform/backend/internal/data/execution/gormrepo"

	"gorm.io/gorm"
)

type Manager struct{ database *gorm.DB }

func New(database *gorm.DB) *Manager { return &Manager{database: database} }

var _ workflow.TransactionManager = (*Manager)(nil)

func (manager *Manager) Within(ctx context.Context, operation func(workflow.Participants) error) error {
	if manager == nil || manager.database == nil || operation == nil {
		return fmt.Errorf("Workflow transaction dependencies are required")
	}
	return manager.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return operation(workflow.Participants{
			Collaboration: collaborationgorm.New(tx),
			Execution:     executiongorm.New(tx),
			Approval:      approvalgorm.New(tx),
		})
	})
}
