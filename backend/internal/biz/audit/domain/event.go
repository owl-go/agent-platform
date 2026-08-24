package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	ID             int64
	OrganizationID string
	TeamID         string
	ActorUserID    string
	Outcome        string
	Action         string
	ResourceType   string
	ResourceID     string
	Details        json.RawMessage
	CreatedAt      time.Time
}

type Query struct {
	OrganizationID string
	TeamID         string
	Action         string
	ResourceType   string
	ResourceID     string
	ActorUserID    string
	Outcome        string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Limit          int
}

func Restore(event Event) (Event, error) {
	if event.ID <= 0 || event.OrganizationID == "" || event.Action == "" || event.ResourceType == "" || event.ResourceID == "" || event.CreatedAt.IsZero() || !json.Valid(event.Details) {
		return Event{}, fmt.Errorf("invalid persisted Audit Event")
	}
	event.Details = append(json.RawMessage(nil), event.Details...)
	return event, nil
}

type Repository interface {
	Search(context.Context, Query) ([]Event, error)
}
