package domain

import "context"

type Repository interface {
	Create(context.Context, Provider) error
	Get(context.Context, string, string) (Provider, error)
	List(context.Context, string) ([]Provider, error)
	UpdateStatus(context.Context, Provider, int64) error
}

type BindingRepository interface {
	CreateBinding(context.Context, RepositoryBinding) error
	GetBinding(context.Context, string, string, string) (RepositoryBinding, error)
	ListBindings(context.Context, string, string) ([]RepositoryBinding, error)
	UpdateBinding(context.Context, RepositoryBinding, int64) error
	UpdateBindingValidation(context.Context, RepositoryBinding, int64) error
}
