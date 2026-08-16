package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/conformanceartifact"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/aliyunoss"
	minioadapter "agent-platform/backend/internal/objectstore/minio"
	"agent-platform/backend/internal/objectstore/providerfactory"
)

type options struct {
	action   string
	provider string
	source   string
	key      string
	report   string
}

type artifactReport struct {
	Action   string    `json:"action"`
	Provider string    `json:"provider,omitempty"`
	Key      string    `json:"key"`
	Size     int64     `json:"size"`
	SHA256   string    `json:"sha256"`
	Files    int       `json:"files,omitempty"`
	Entries  int       `json:"entries,omitempty"`
	At       time.Time `json:"at"`
}

func main() {
	var opts options
	flag.StringVar(&opts.action, "action", "", "upload or restore")
	flag.StringVar(&opts.provider, "provider", "", "minio or aliyun_oss")
	flag.StringVar(&opts.source, "source", "", "source directory for upload or empty target directory for restore")
	flag.StringVar(&opts.key, "key", "", "object storage key")
	flag.StringVar(&opts.report, "report", "", "JSON report output path")
	flag.Parse()
	if opts.action == "" || opts.provider == "" || opts.source == "" || opts.key == "" || opts.report == "" {
		log.Fatal("action, provider, source, key, and report are required")
	}
	provider, err := providerFromEnvironment(opts.provider)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var result artifactReport
	switch opts.action {
	case "upload":
		result, err = upload(ctx, provider, opts.source, opts.key)
	case "restore":
		result, err = restore(ctx, provider, opts.source, opts.key)
	default:
		err = fmt.Errorf("unsupported action %q", opts.action)
	}
	if err != nil {
		log.Fatal(err)
	}
	result.Provider = opts.provider
	if err := writeJSON(opts.report, result); err != nil {
		log.Fatal(err)
	}
}

func upload(ctx context.Context, provider objectstore.Provider, source, key string) (artifactReport, error) {
	archive, err := os.CreateTemp("", "agent-conformance-artifact-*.tar")
	if err != nil {
		return artifactReport{}, fmt.Errorf("create temporary archive: %w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	if err := archive.Chmod(0o600); err != nil {
		_ = archive.Close()
		return artifactReport{}, err
	}
	metadata, archiveErr := conformanceartifact.Archive(source, archive)
	closeErr := archive.Close()
	if archiveErr != nil || closeErr != nil {
		return artifactReport{}, errors.Join(archiveErr, closeErr)
	}
	reader, err := os.Open(archivePath)
	if err != nil {
		return artifactReport{}, err
	}
	_, putErr := provider.Put(ctx, key, reader, objectstore.PutOptions{
		Size: metadata.Size, SHA256: metadata.SHA256, ContentType: "application/x-tar",
		Metadata: map[string]string{"artifact-kind": "production-conformance"},
	})
	closeErr = reader.Close()
	if putErr != nil || closeErr != nil {
		return artifactReport{}, errors.Join(putErr, closeErr)
	}
	verified, err := verifyObject(ctx, provider, key)
	if err != nil {
		return artifactReport{}, err
	}
	if verified.SHA256 != metadata.SHA256 || verified.Size != metadata.Size {
		return artifactReport{}, fmt.Errorf("uploaded object verification mismatch")
	}
	return artifactReport{
		Action: "uploaded", Key: key, Size: metadata.Size, SHA256: metadata.SHA256,
		Files: metadata.Files, Entries: metadata.Entries, At: time.Now().UTC(),
	}, nil
}

func restore(ctx context.Context, provider objectstore.Provider, target, key string) (artifactReport, error) {
	reader, metadata, err := provider.Get(ctx, key)
	if err != nil {
		return artifactReport{}, err
	}
	digest := sha256.New()
	counted := &countingReader{next: io.TeeReader(reader, digest)}
	restoreErr := conformanceartifact.Restore(counted, target)
	closeErr := reader.Close()
	if restoreErr != nil || closeErr != nil {
		return artifactReport{}, errors.Join(restoreErr, closeErr)
	}
	checksum := hex.EncodeToString(digest.Sum(nil))
	if checksum != metadata.SHA256 || counted.count != metadata.Size {
		return artifactReport{}, fmt.Errorf("restored object checksum mismatch")
	}
	return artifactReport{Action: "restored", Key: key, Size: counted.count, SHA256: checksum, At: time.Now().UTC()}, nil
}

func verifyObject(ctx context.Context, provider objectstore.Provider, key string) (objectstore.Object, error) {
	reader, metadata, err := provider.Get(ctx, key)
	if err != nil {
		return objectstore.Object{}, err
	}
	digest := sha256.New()
	count, copyErr := io.Copy(digest, reader)
	closeErr := reader.Close()
	if copyErr != nil || closeErr != nil {
		return objectstore.Object{}, errors.Join(copyErr, closeErr)
	}
	if count != metadata.Size || hex.EncodeToString(digest.Sum(nil)) != metadata.SHA256 {
		return objectstore.Object{}, fmt.Errorf("object storage read-after-write checksum mismatch")
	}
	return metadata, nil
}

func providerFromEnvironment(name string) (objectstore.Provider, error) {
	switch name {
	case "minio":
		secure, err := strconv.ParseBool(environmentDefault("MINIO_SECURE", "true"))
		if err != nil {
			return nil, fmt.Errorf("parse MINIO_SECURE: %w", err)
		}
		return providerfactory.New(providerfactory.Config{Provider: providerfactory.ProviderMinIO, MinIO: minioadapter.Config{
			Endpoint: os.Getenv("MINIO_ENDPOINT"), AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET_KEY"), SessionToken: os.Getenv("MINIO_SESSION_TOKEN"),
			Bucket: os.Getenv("MINIO_BUCKET"), Secure: secure, Prefix: os.Getenv("MINIO_PREFIX"),
		}})
	case "aliyun_oss":
		return providerfactory.New(providerfactory.Config{Provider: providerfactory.ProviderAliyunOSS, AliyunOSS: aliyunoss.Config{
			Endpoint: os.Getenv("ALIYUN_OSS_ENDPOINT"), AccessKey: os.Getenv("ALIYUN_OSS_ACCESS_KEY"),
			SecretKey: os.Getenv("ALIYUN_OSS_SECRET_KEY"), SessionToken: os.Getenv("ALIYUN_OSS_SESSION_TOKEN"),
			Bucket: os.Getenv("ALIYUN_OSS_BUCKET"), Prefix: os.Getenv("ALIYUN_OSS_PREFIX"),
		}})
	default:
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
}

func environmentDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o600)
}

type countingReader struct {
	next  io.Reader
	count int64
}

func (r *countingReader) Read(value []byte) (int, error) {
	count, err := r.next.Read(value)
	r.count += int64(count)
	return count, err
}
