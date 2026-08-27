package platform

import (
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/aliyunoss"
	minioadapter "agent-platform/backend/internal/objectstore/minio"
	"agent-platform/backend/internal/objectstore/providerfactory"
	"agent-platform/backend/internal/platformconfig"

	"github.com/google/wire"
)

var APIProviderSet = wire.NewSet(NewObjectStore)

func NewObjectStore(config platformconfig.Config) (objectstore.Provider, error) {
	return providerfactory.New(providerfactory.Config{
		Provider: providerfactory.Name(config.ObjectStore.Provider),
		MinIO: minioadapter.Config{
			Endpoint: config.ObjectStore.MinIO.Endpoint, AccessKey: config.ObjectStore.MinIO.AccessKey,
			SecretKey: config.ObjectStore.MinIO.SecretKey, Bucket: config.ObjectStore.MinIO.Bucket, Secure: config.ObjectStore.MinIO.Secure,
		},
		AliyunOSS: aliyunoss.Config{
			Endpoint: config.ObjectStore.AliyunOSS.Endpoint, AccessKey: config.ObjectStore.AliyunOSS.AccessKeyID,
			SecretKey: config.ObjectStore.AliyunOSS.AccessKeySecret, Bucket: config.ObjectStore.AliyunOSS.Bucket, Prefix: config.ObjectStore.AliyunOSS.Prefix,
		},
	})
}
