package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	catalogapp "github.com/yywencs/courseforge/internal/catalog/application"
	"github.com/yywencs/courseforge/internal/platform/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Store struct {
	client *minio.Client
	core   minio.Core
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
	return &S3Store{
		client: client,
		core:   minio.Core{Client: client},
		bucket: strings.TrimSpace(cfg.Bucket),
	}, nil
}

func (s *S3Store) CreateMultipartUpload(ctx context.Context, objectKey, contentType string) (string, error) {
	return s.core.NewMultipartUpload(ctx, s.bucket, objectKey, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

func (s *S3Store) PresignUploadPart(
	ctx context.Context,
	objectKey string,
	multipartUploadID string,
	partNumber int,
	expiry time.Duration,
) (string, error) {
	query := url.Values{
		"uploadId":   []string{multipartUploadID},
		"partNumber": []string{strconv.Itoa(partNumber)},
	}
	value, err := s.client.Presign(ctx, http.MethodPut, s.bucket, objectKey, expiry, query)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

func (s *S3Store) ListUploadedParts(
	ctx context.Context,
	objectKey string,
	multipartUploadID string,
) ([]catalogapp.UploadedPart, error) {
	parts := make([]catalogapp.UploadedPart, 0)
	marker := 0
	for {
		result, err := s.core.ListObjectParts(
			ctx, s.bucket, objectKey, multipartUploadID, marker, 1000,
		)
		if err != nil {
			if isMissingMultipartUpload(err) {
				return nil, catalogapp.ErrMultipartUploadNotFound
			}
			return nil, err
		}
		for _, part := range result.ObjectParts {
			parts = append(parts, catalogapp.UploadedPart{
				PartNumber: part.PartNumber,
				ETag:       part.ETag,
				Size:       part.Size,
			})
		}
		if !result.IsTruncated {
			break
		}
		marker = result.NextPartNumberMarker
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	return parts, nil
}

func (s *S3Store) CompleteMultipartUpload(
	ctx context.Context,
	objectKey string,
	multipartUploadID string,
	parts []catalogapp.UploadedPart,
) error {
	completeParts := make([]minio.CompletePart, 0, len(parts))
	for _, part := range parts {
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}
	_, err := s.core.CompleteMultipartUpload(
		ctx, s.bucket, objectKey, multipartUploadID, completeParts, minio.PutObjectOptions{},
	)
	if isMissingMultipartUpload(err) {
		return catalogapp.ErrMultipartUploadNotFound
	}
	return err
}

// AbortMultipartUpload 幂等终止尚未合并的分片上传会话。
func (s *S3Store) AbortMultipartUpload(ctx context.Context, objectKey, multipartUploadID string) error {
	err := s.core.AbortMultipartUpload(ctx, s.bucket, objectKey, multipartUploadID)
	if isMissingMultipartUpload(err) {
		return nil
	}
	return err
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

func isMissingMultipartUpload(err error) bool {
	if err == nil {
		return false
	}
	response := minio.ToErrorResponse(err)
	return response.Code == "NoSuchUpload"
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
