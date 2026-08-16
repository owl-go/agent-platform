package domain

import "context"

type Repository interface {
	Create(context.Context, RuntimeImage) error
	Get(context.Context, string) (RuntimeImage, error)
	List(context.Context) ([]RuntimeImage, error)
	UpdateStatus(context.Context, RuntimeImage, int64) error
}
