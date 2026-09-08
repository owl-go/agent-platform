package aicreation

import (
	"context"
	"errors"
	"io"

	aicreationapplication "agent-platform/backend/internal/biz/aicreation/application"
	aicreationdomain "agent-platform/backend/internal/biz/aicreation/domain"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	aicreationrepo "agent-platform/backend/internal/data/aicreation/gormrepo"
	"agent-platform/backend/internal/data/aicreation/openaiimages"
	"agent-platform/backend/internal/data/aicreation/promptoptimizer"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/secretcrypto"
)

func NewApplication(database *gormdb.Database, creditsRepository *creditsrepo.Repository, credits *creditsapplication.Service, box *secretcrypto.Box, objects objectstore.Provider) (*aicreationapplication.Service, error) {
	resolver, err := openaiimages.NewDatabaseConnectionResolver(database.ORM(), box)
	if err != nil {
		return nil, err
	}
	provider, err := openaiimages.New(resolver, openaiimages.NewObjectSources(objects), nil)
	if err != nil {
		return nil, err
	}
	service, err := aicreationapplication.New(aicreationrepo.New(database.ORM(), creditsRepository), provider, objectPort{objects}, nil)
	if err != nil {
		return nil, err
	}
	optimizer, err := promptoptimizer.New(resolver, nil)
	if err != nil {
		return nil, err
	}
	if err := service.EnablePromptOptimization(optimizer, creditPort{credits}); err != nil {
		return nil, err
	}
	return service, nil
}

type creditPort struct{ service *creditsapplication.Service }

func (port creditPort) Admit(ctx context.Context, ownerID, executionID, timezone string, candidate aicreationdomain.PromptOptimizationCandidate) (aicreationapplication.TextCreditAdmission, error) {
	admission, err := port.service.Admit(ctx, creditsapplication.AdmissionRequest{UserID: ownerID, ExecutionID: executionID, StagePosition: 1, Timezone: timezone, ProviderType: candidate.ProviderType, Protocol: candidate.Protocol, ModelID: candidate.ModelID})
	return aicreationapplication.TextCreditAdmission{Value: admission}, err
}

func (port creditPort) SettleText(ctx context.Context, admission aicreationapplication.TextCreditAdmission, inputTokens, outputTokens int64) error {
	value, ok := admission.Value.(creditsdomain.Admission)
	if !ok {
		return errors.New("invalid text Credit admission")
	}
	_, err := port.service.Settle(ctx, creditsapplication.SettlementRequest{Admission: value, Usage: creditsdomain.Usage{InputTokens: inputTokens, OutputTokens: outputTokens, Known: true}})
	return err
}

func (port creditPort) Abort(ctx context.Context, admission aicreationapplication.TextCreditAdmission) error {
	value, ok := admission.Value.(creditsdomain.Admission)
	if !ok {
		return errors.New("invalid text Credit admission")
	}
	return port.service.Abort(ctx, value)
}

type objectPort struct{ provider objectstore.Provider }

func (port objectPort) Put(ctx context.Context, key string, body io.Reader, metadata aicreationapplication.ObjectMetadata) error {
	_, err := port.provider.Put(ctx, key, body, objectstore.PutOptions{Size: metadata.Size, SHA256: metadata.SHA256, ContentType: metadata.ContentType})
	return err
}

func (port objectPort) Delete(ctx context.Context, key string) error {
	return port.provider.Delete(ctx, key)
}

func (port objectPort) Get(ctx context.Context, key string) (io.ReadCloser, aicreationapplication.ObjectMetadata, error) {
	reader, object, err := port.provider.Get(ctx, key)
	return reader, aicreationapplication.ObjectMetadata{Size: object.Size, SHA256: object.SHA256, ContentType: object.ContentType}, err
}
