package domain

import (
	"context"
	"time"
)

type Repository interface {
	Create(context.Context, RuntimeImage) error
	Get(context.Context, string, string) (RuntimeImage, error)
	List(context.Context, PageQuery) (Page, error)
	UpdateStatus(context.Context, RuntimeImage, int64) error
}

type PageQuery struct {
	OrganizationID string
	Limit          int
	After          *PageCursor
}

type PageCursor struct {
	Runtime   Runtime
	CreatedAt time.Time
	ID        string
}

type Page struct {
	Items   []RuntimeImage
	HasMore bool
}
