package knowledgebase

import (
	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/knowledgebase/ragflow"
	"agent-platform/backend/internal/knowledgebase/retrieval"
	"agent-platform/backend/internal/platformconfig"
)

// NewSearcher returns nil while retrieval is explicitly disabled.
func NewSearcher(config platformconfig.Config, database *gormdb.Database) (retrieval.Searcher, error) {
	if config.Retrieval.Provider == "" {
		return nil, nil
	}
	repository := workspacerepo.New(database.ORM(), nil)
	provider, err := NewProvider(config, repository)
	if err != nil {
		return nil, err
	}
	return retrieval.New(repository, provider)
}
func NewProvider(config platformconfig.Config, store ragflow.Store) (*ragflow.Client, error) {
	if err := config.Retrieval.Validate(); err != nil {
		return nil, err
	}
	if config.Retrieval.Provider == "" {
		return nil, nil
	}
	r := config.Retrieval.RAGFlow
	return ragflow.New(ragflow.Config{Endpoint: r.Endpoint, APIKey: r.APIKey, DeploymentID: r.DeploymentID, EmbeddingModel: r.EmbeddingModel, RequestTimeout: r.RequestTimeout.Value(), ParseTimeout: r.ParseTimeout.Value()}, store, nil)
}
