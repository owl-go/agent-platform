package aliyunoss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"agent-platform/backend/internal/objectstore"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	defaultPartSize = 8 * 1024 * 1024
	checksumKey     = "agent-sha256"
	metadataKey     = "agent-metadata"
)

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	SessionToken string
	Bucket       string
	Prefix       string
	PartSize     int64
}

type Provider struct {
	bucket   *oss.Bucket
	prefix   string
	partSize int64
	now      func() time.Time
}

func New(config Config) (*Provider, error) {
	if config.Endpoint == "" || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("Aliyun OSS endpoint, bucket, access key, and secret key are required")
	}
	prefix, err := objectstore.NormalizePrefix(config.Prefix)
	if err != nil {
		return nil, err
	}
	clientOptions := make([]oss.ClientOption, 0, 1)
	if config.SessionToken != "" {
		clientOptions = append(clientOptions, oss.SecurityToken(config.SessionToken))
	}
	client, err := oss.New(config.Endpoint, config.AccessKey, config.SecretKey, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("create Aliyun OSS client: %w", err)
	}
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("open Aliyun OSS bucket: %w", err)
	}
	partSize := config.PartSize
	if partSize == 0 {
		partSize = defaultPartSize
	}
	if partSize < 100*1024 {
		return nil, fmt.Errorf("Aliyun OSS multipart size must be at least 100 KiB")
	}
	return &Provider{
		bucket:   bucket,
		prefix:   prefix,
		partSize: partSize,
		now:      time.Now,
	}, nil
}

func (p *Provider) Put(ctx context.Context, key string, body io.Reader, options objectstore.PutOptions) (objectstore.Object, error) {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return objectstore.Object{}, err
	}
	upload, err := objectstore.PrepareUpload(ctx, body, options)
	if err != nil {
		return objectstore.Object{}, err
	}
	defer upload.Cleanup()
	encodedMetadata, err := objectstore.EncodeMetadata(options.Metadata)
	if err != nil {
		return objectstore.Object{}, err
	}
	uploadOptions := []oss.Option{
		oss.WithContext(ctx),
		oss.ContentType(options.ContentType),
		oss.Meta(checksumKey, options.SHA256),
		oss.Meta(metadataKey, encodedMetadata),
	}
	if options.Size <= p.partSize {
		if err := p.bucket.PutObject(physicalKey, upload.File, uploadOptions...); err != nil {
			return objectstore.Object{}, mapError("put Aliyun OSS object", err)
		}
	} else if err := p.putMultipart(ctx, physicalKey, upload.Path, options.Size, uploadOptions); err != nil {
		return objectstore.Object{}, err
	}
	return p.Stat(ctx, key)
}

func (p *Provider) putMultipart(ctx context.Context, key, filePath string, size int64, options []oss.Option) (returnErr error) {
	initResult, err := p.bucket.InitiateMultipartUpload(key, options...)
	if err != nil {
		return mapError("initiate Aliyun OSS multipart upload", err)
	}
	completed := false
	defer func() {
		if !completed {
			abortErr := p.bucket.AbortMultipartUpload(initResult, oss.WithContext(context.WithoutCancel(ctx)))
			if abortErr != nil {
				returnErr = errors.Join(returnErr, mapError("abort Aliyun OSS multipart upload", abortErr))
			}
		}
	}()

	parts := make([]oss.UploadPart, 0, (size+p.partSize-1)/p.partSize)
	for offset, partNumber := int64(0), 1; offset < size; offset, partNumber = offset+p.partSize, partNumber+1 {
		partSize := min(p.partSize, size-offset)
		part, err := p.bucket.UploadPartFromFile(initResult, filePath, offset, partSize, partNumber, oss.WithContext(ctx))
		if err != nil {
			return mapError("upload Aliyun OSS part", err)
		}
		parts = append(parts, part)
	}
	if _, err := p.bucket.CompleteMultipartUpload(initResult, parts, oss.WithContext(ctx)); err != nil {
		return mapError("complete Aliyun OSS multipart upload", err)
	}
	completed = true
	return nil
}

func (p *Provider) Get(ctx context.Context, key string) (io.ReadCloser, objectstore.Object, error) {
	metadata, err := p.Stat(ctx, key)
	if err != nil {
		return nil, objectstore.Object{}, err
	}
	physicalKey, _ := p.physicalKey(key)
	reader, err := p.bucket.GetObject(physicalKey, oss.WithContext(ctx))
	if err != nil {
		return nil, objectstore.Object{}, mapError("get Aliyun OSS object", err)
	}
	return reader, metadata, nil
}

func (p *Provider) Stat(ctx context.Context, key string) (objectstore.Object, error) {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return objectstore.Object{}, err
	}
	headers, err := p.bucket.GetObjectDetailedMeta(physicalKey, oss.WithContext(ctx))
	if err != nil {
		return objectstore.Object{}, mapError("stat Aliyun OSS object", err)
	}
	size, err := strconv.ParseInt(headers.Get(oss.HTTPHeaderContentLength), 10, 64)
	if err != nil {
		return objectstore.Object{}, fmt.Errorf("parse Aliyun OSS object size: %w", err)
	}
	lastModified, err := http.ParseTime(headers.Get(oss.HTTPHeaderLastModified))
	if err != nil {
		return objectstore.Object{}, fmt.Errorf("parse Aliyun OSS modification time: %w", err)
	}
	metadata, err := objectstore.DecodeMetadata(headers.Get(oss.HTTPHeaderOssMetaPrefix + metadataKey))
	if err != nil {
		return objectstore.Object{}, err
	}
	return objectstore.Object{
		Key:          key,
		Size:         size,
		SHA256:       headers.Get(oss.HTTPHeaderOssMetaPrefix + checksumKey),
		ContentType:  headers.Get(oss.HTTPHeaderContentType),
		Metadata:     metadata,
		LastModified: lastModified,
	}, nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return err
	}
	if err := p.bucket.DeleteObject(physicalKey, oss.WithContext(ctx)); err != nil {
		return mapError("delete Aliyun OSS object", err)
	}
	return nil
}

func (p *Provider) PresignGet(ctx context.Context, key string, expiresIn time.Duration) (objectstore.SignedURL, error) {
	if expiresIn <= 0 || expiresIn > 7*24*time.Hour {
		return objectstore.SignedURL{}, objectstore.ErrInvalidExpiry
	}
	if _, err := p.Stat(ctx, key); err != nil {
		return objectstore.SignedURL{}, err
	}
	physicalKey, _ := p.physicalKey(key)
	signed, err := p.bucket.SignURL(physicalKey, oss.HTTPGet, int64(expiresIn/time.Second))
	if err != nil {
		return objectstore.SignedURL{}, mapError("sign Aliyun OSS object", err)
	}
	return objectstore.SignedURL{URL: signed, ExpiresAt: p.now().UTC().Add(expiresIn)}, nil
}

func (p *Provider) DeleteExpired(ctx context.Context, query objectstore.LifecycleQuery) (int, error) {
	if query.Before.IsZero() {
		return 0, fmt.Errorf("lifecycle cutoff is required")
	}
	prefix := p.prefixed(query.Prefix)
	marker := ""
	deleted := 0
	for {
		result, err := p.bucket.ListObjects(
			oss.Prefix(prefix),
			oss.Marker(marker),
			oss.MaxKeys(1000),
			oss.WithContext(ctx),
		)
		if err != nil {
			return deleted, mapError("list Aliyun OSS objects", err)
		}
		for _, info := range result.Objects {
			if !info.LastModified.Before(query.Before) {
				continue
			}
			if err := p.bucket.DeleteObject(info.Key, oss.WithContext(ctx)); err != nil {
				return deleted, mapError("delete expired Aliyun OSS object", err)
			}
			deleted++
		}
		if !result.IsTruncated {
			return deleted, nil
		}
		marker = result.NextMarker
	}
}

func (p *Provider) physicalKey(key string) (string, error) {
	if err := objectstore.ValidateKey(key); err != nil {
		return "", err
	}
	return p.prefixed(key), nil
}

func (p *Provider) prefixed(key string) string {
	if p.prefix == "" {
		return key
	}
	if key == "" {
		return p.prefix + "/"
	}
	return p.prefix + "/" + key
}

func mapError(operation string, err error) error {
	var serviceError oss.ServiceError
	if errors.As(err, &serviceError) && (serviceError.Code == "NoSuchKey" || serviceError.StatusCode == http.StatusNotFound) {
		return fmt.Errorf("%s: %w", operation, objectstore.ErrNotFound)
	}
	var serviceErrorPointer *oss.ServiceError
	if errors.As(err, &serviceErrorPointer) && (serviceErrorPointer.Code == "NoSuchKey" || serviceErrorPointer.StatusCode == http.StatusNotFound) {
		return fmt.Errorf("%s: %w", operation, objectstore.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
