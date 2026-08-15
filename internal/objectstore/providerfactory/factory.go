package providerfactory

import (
	"fmt"

	"agent-platform/internal/objectstore"
	"agent-platform/internal/objectstore/aliyunoss"
	"agent-platform/internal/objectstore/memory"
	minioadapter "agent-platform/internal/objectstore/minio"
)

type Name string

const (
	ProviderMemory    Name = "memory"
	ProviderMinIO     Name = "minio"
	ProviderAliyunOSS Name = "aliyun_oss"
)

type Config struct {
	Provider  Name
	MinIO     minioadapter.Config
	AliyunOSS aliyunoss.Config
}

func New(config Config) (objectstore.Provider, error) {
	switch config.Provider {
	case ProviderMemory:
		return memory.New(), nil
	case ProviderMinIO:
		return minioadapter.New(config.MinIO)
	case ProviderAliyunOSS:
		return aliyunoss.New(config.AliyunOSS)
	default:
		return nil, fmt.Errorf("object storage provider %q is unsupported", config.Provider)
	}
}
