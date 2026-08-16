package domain

import "context"

type Repository interface {
	CreateAgent(context.Context, Agent) error
	GetAgent(context.Context, string, string, string) (Agent, error)
	ListAgents(context.Context, string, string) ([]Agent, error)
	UpdateAgent(context.Context, Agent, int64) error

	CreateDraft(context.Context, DraftRegistration) (Draft, error)
	GetDraft(context.Context, string, string) (Draft, error)
	ListDrafts(context.Context, string) ([]Draft, error)
	UpdateDraft(context.Context, Draft, int64) error

	CreateApproval(context.Context, ReleaseApproval) error
	GetApprovalByDraft(context.Context, string) (ReleaseApproval, error)
	UpdateApproval(context.Context, ReleaseApproval, int64) error

	CreateRelease(context.Context, ReleaseRegistration) (Release, error)
	GetRelease(context.Context, string, string) (Release, error)
	ListReleases(context.Context, string) ([]Release, error)
	UpdateReleaseStatus(context.Context, Release, int64) error
}
