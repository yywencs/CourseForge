package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	catalogapp "github.com/yywencs/courseforge/internal/catalog/application"
	"github.com/yywencs/courseforge/internal/platform/config"
)

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		value      string
		configured bool
		wantHost   string
		wantSecure bool
	}{
		{value: "127.0.0.1:9000", configured: false, wantHost: "127.0.0.1:9000"},
		{value: "https://s3.example.com", wantHost: "s3.example.com", wantSecure: true},
	}
	for _, testCase := range tests {
		host, secure, err := resolveEndpoint(testCase.value, testCase.configured)
		if err != nil || host != testCase.wantHost || secure != testCase.wantSecure {
			t.Fatalf("resolveEndpoint(%q) = %q, %v, %v", testCase.value, host, secure, err)
		}
	}
}

func TestS3StorePresignedRoundTrip(t *testing.T) {
	if os.Getenv("COURSEFORGE_MINIO_INTEGRATION") != "1" {
		t.Skip("set COURSEFORGE_MINIO_INTEGRATION=1 to run against local MinIO")
	}
	store, err := NewS3Store(config.ObjectStorageConfig{
		Endpoint: "127.0.0.1:9000", AccessKey: "courseforge",
		SecretKey: "courseforge-local-password", Bucket: "courseforge",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	objectKey := "integration-tests/" + time.Now().Format("20060102150405.000000000") + ".mp4"
	t.Cleanup(func() {
		_ = store.DeleteObject(context.Background(), objectKey)
	})

	multipartUploadID, err := store.CreateMultipartUpload(ctx, objectKey, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.AbortMultipartUpload(context.Background(), objectKey, multipartUploadID)
	})
	uploadURL, err := store.PresignUploadPart(ctx, objectKey, multipartUploadID, 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewBufferString("video-data"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "video/mp4")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode/100 != 2 {
		t.Fatalf("upload status = %d", response.StatusCode)
	}
	parts, err := store.ListUploadedParts(ctx, objectKey, multipartUploadID)
	if err != nil || len(parts) != 1 || parts[0].Size != int64(len("video-data")) {
		t.Fatalf("ListUploadedParts() = %#v, %v", parts, err)
	}
	if err := store.CompleteMultipartUpload(ctx, objectKey, multipartUploadID, parts); err != nil {
		t.Fatalf("CompleteMultipartUpload() error = %v", err)
	}

	info, err := store.StatObject(ctx, objectKey)
	if err != nil || info.Size != int64(len("video-data")) || info.ContentType != "video/mp4" {
		t.Fatalf("StatObject() = %#v, %v", info, err)
	}
	playURL, err := store.PresignPlayback(ctx, objectKey, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	playResponse, err := http.Get(playURL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(playResponse.Body)
	_ = playResponse.Body.Close()
	if readErr != nil || playResponse.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "video-data" {
		t.Fatalf("playback status = %d, body = %q, error = %v", playResponse.StatusCode, body, readErr)
	}
	if err := store.DeleteObject(ctx, objectKey); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
	if _, err := store.StatObject(ctx, objectKey); !errors.Is(err, catalogapp.ErrStoredObjectNotFound) {
		t.Fatalf("StatObject() after delete error = %v, want %v", err, catalogapp.ErrStoredObjectNotFound)
	}
	if err := store.DeleteObject(ctx, objectKey); err != nil {
		t.Fatalf("second DeleteObject() error = %v", err)
	}
}
