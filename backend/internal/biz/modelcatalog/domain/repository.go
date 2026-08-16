package domain

import "context"

type Repository interface {
	CreateCredential(context.Context, CredentialProfile) error
	GetCredential(context.Context, string, string) (CredentialProfile, error)
	ListCredentials(context.Context, string) ([]CredentialProfile, error)
	UpdateCredentialStatus(context.Context, CredentialProfile, int64) error

	CreateModel(context.Context, ConfiguredModel) error
	GetModel(context.Context, string, string) (ConfiguredModel, error)
	ListModels(context.Context, string) ([]ConfiguredModel, error)
	UpdateModelStatus(context.Context, ConfiguredModel, int64) error
}
