package bootstrap

import (
	enrollmentintegration "github.com/yywencs/courseforge/internal/enrollment/integration"
	notificationasync "github.com/yywencs/courseforge/internal/notification/async"
	notificationmysql "github.com/yywencs/courseforge/internal/notification/infrastructure/mysql"
)

func registerNotificationListeners(runtime *apiRuntime) {
	runtime.consumer.RegisterListener(
		enrollmentintegration.SelectionNotificationTopic,
		notificationasync.NewSelectionListener(notificationmysql.NewRepository(runtime.db)),
	)
}
