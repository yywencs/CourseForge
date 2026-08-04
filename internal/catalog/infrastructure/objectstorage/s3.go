package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	catalogapp "prizeforge/internal/catalog/application"
	"prizeforge/internal/platform/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Store struct {
	client *minio.Client
	bucket string
}

var _ catalogapp.ObjectStorage = (*S3Store)(nil)

func NewS3Store(cfg config.ObjectStorageConfig) (*S3Store, error) {
	endpoint, secure, err := resolveEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}
	return &S3Store{client: client, bucket: strings.TrimSpace(cfg.Bucket)}, nil
}

func (s *S3Store) PresignUpload(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	value, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expiry)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

func (s *S3Store) StatObject(ctx context.Context, objectKey string) (catalogapp.StoredObject, error) {
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == 404 {
			return catalogapp.StoredObject{}, catalogapp.ErrStoredObjectNotFound
		}
		return catalogapp.StoredObject{}, err
	}
	return catalogapp.StoredObject{Size: info.Size, ContentType: info.ContentType}, nil
}

func (s *S3Store) PresignPlayback(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	value, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, expiry, nil)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

// DeleteObject 幂等删除精确对象键；对象已经不存在时同样视为成功。
func (s *S3Store) DeleteObject(ctx context.Context, objectKey string) error {
	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	if err == nil {
		return nil
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == 404 {
		return nil
	}
	return err
}

func resolveEndpoint(value string, configuredSecure bool) (string, bool, error) {
	value = strings.TrimSpace(value)
	// MinIO SDK 需要不带协议的 host；若配置显式携带协议，则以协议决定是否启用 TLS。
	if !strings.Contains(value, "://") {
		if value == "" {
			return "", false, errors.New("object storage endpoint is empty")
		}
		return value, configuredSecure, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false, errors.New("object storage endpoint is invalid")
	}
	return parsed.Host, parsed.Scheme == "https", nil
}
