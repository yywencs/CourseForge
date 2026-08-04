package bootstrap

import (
	"time"

	applicationcatalog "prizeforge/internal/catalog/application"
	catalogasync "prizeforge/internal/catalog/async"
	catalogstorage "prizeforge/internal/catalog/infrastructure/objectstorage"
	"prizeforge/internal/platform/config"
	"prizeforge/internal/platform/taskqueue"
)

func newCatalogService(
	repository applicationcatalog.Repository,
	cfg config.ObjectStorageConfig,
) (*applicationcatalog.Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		// 对象存储关闭时保留课程目录能力，仅视频上传和播放返回服务不可用。
		return applicationcatalog.NewService(repository), nil
	}
	storage, err := catalogstorage.NewS3Store(cfg)
	if err != nil {
		return nil, err
	}
	return applicationcatalog.NewService(
		repository,
		applicationcatalog.WithVideoStorage(storage, applicationcatalog.VideoPolicy{
			UploadURLTTL:      cfg.UploadURLTTL,
			PlaybackURLTTL:    cfg.PlaybackURLTTL,
			MaxVideoSizeBytes: cfg.MaxVideoSizeBytes,
		}),
	), nil
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
