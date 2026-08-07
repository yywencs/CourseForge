package bootstrap

import (
	"time"

	applicationcatalog "github.com/yywencs/courseforge/internal/catalog/application"
	catalogasync "github.com/yywencs/courseforge/internal/catalog/async"
	catalogstorage "github.com/yywencs/courseforge/internal/catalog/infrastructure/objectstorage"
	"github.com/yywencs/courseforge/internal/platform/config"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"
)

func newCatalogService(
	repository applicationcatalog.Repository,
	cfg config.ObjectStorageConfig,
	options ...applicationcatalog.ServiceOption,
) (*applicationcatalog.Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		// 对象存储关闭时保留课程目录能力，仅视频上传和播放返回服务不可用。
		return applicationcatalog.NewService(repository, options...), nil
	}
	storage, err := catalogstorage.NewS3Store(cfg)
	if err != nil {
		return nil, err
	}
	options = append(options, applicationcatalog.WithVideoStorage(storage, applicationcatalog.VideoPolicy{
		UploadURLTTL:      cfg.UploadURLTTL,
		PlaybackURLTTL:    cfg.PlaybackURLTTL,
		MaxVideoSizeBytes: cfg.MaxVideoSizeBytes,
	}))
	return applicationcatalog.NewService(repository, options...), nil
}

func newCatalogScheduledHandlers(
	cfg *config.Config,
	catalogService *applicationcatalog.Service,
) []taskqueue.ScheduledHandler {
	if !cfg.Data.ObjectStorage.Enabled {
		return nil
	}
	videoUploadCleanupJob := catalogasync.NewVideoUploadCleanupJob(catalogService, 100, time.Hour)
	return []taskqueue.ScheduledHandler{
		taskqueue.NewScheduledHandler(
			catalogasync.TaskTypeVideoUploadCleanup,
			"@every 5m",
			videoUploadCleanupJob.ProcessTask,
		),
	}
}
