package bootstrap

import (
	enrollmentapp "prizeforge/internal/enrollment/application"
	enrollmentasync "prizeforge/internal/enrollment/async"
	enrollmentobservability "prizeforge/internal/enrollment/infrastructure/observability"
	enrollmentrepo "prizeforge/internal/enrollment/infrastructure/persistence"
	enrollmenthttp "prizeforge/internal/enrollment/transport/http"
	"prizeforge/internal/platform/http/middleware"
	"prizeforge/internal/platform/identifier"
	"prizeforge/internal/platform/outbox"
	outboxdispatcher "prizeforge/internal/platform/outbox/dispatcher"
	outboxrepo "prizeforge/internal/platform/outbox/mysql"
	"prizeforge/internal/platform/taskqueue"

	"github.com/gin-gonic/gin"
)

type enrollmentModule struct {
	routes            *enrollmenthttp.Routes
	scheduledHandlers []taskqueue.ScheduledHandler
}

func newEnrollmentModule(runtime *apiRuntime, authMiddleware gin.HandlerFunc) *enrollmentModule {
	ids := identifier.NewOrderIDGenerator()
	observer := enrollmentobservability.NewPrometheusObserver()
	stores := enrollmentrepo.NewStores(runtime.db, runtime.redis, ids)
	selectionResultPublisher := enrollmentasync.NewSelectionResultPublisher(
		stores.Selections,
		runtime.publisher,
	)
	selectionResultRecovery := enrollmentasync.NewSelectionResultRecoveryJob(
		stores.Selections,
		selectionResultPublisher,
	)
	outboxDispatcher := outboxdispatcher.NewOutboxDispatcher(
		outboxrepo.NewRepository(runtime.db),
		runtime.publisher,
	)
	runtime.consumer.RegisterListener(
		runtime.cfg.RabbitMQ.Topic.SelectionResult,
		enrollmentasync.NewSelectionResultListener(stores.Results),
	)

	selectionAdmission := enrollmentapp.NewSelectionAdmissionService(
		stores.Queries,
		stores.Eligibility,
	)
	enrollmentUsecase := enrollmentapp.NewEnrollmentUsecase(
		stores.Queries,
		stores.Selections,
		selectionResultPublisher,
		selectionAdmission,
		ids,
		observer,
	)
	dropEnrollmentUsecase := enrollmentapp.NewDropEnrollmentUsecase(
		stores.Enrollments,
		stores.Projections,
		stores.Repairs,
		observer,
	)
	waitlistUsecase := enrollmentapp.NewWaitlistUsecase(
		stores.Waitlist,
		enrollmentUsecase,
		selectionAdmission,
		ids,
		observer,
	)
	projectionReconciliationUsecase := enrollmentapp.NewProjectionReconciliationUsecase(
		stores.Repairs,
		stores.Projections,
		observer,
	)

	selectionLimiter := middleware.NewSelectionRateLimiter(runtime.cfg.Dcc.RateLimit)
	return &enrollmentModule{
		routes: enrollmenthttp.NewRoutes(
			enrollmentUsecase,
			dropEnrollmentUsecase,
			waitlistUsecase,
			authMiddleware,
			selectionLimiter.Handle,
		),
		scheduledHandlers: []taskqueue.ScheduledHandler{
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeSelectionResultPublish,
				"@every 1s",
				selectionResultRecovery.ProcessTask,
			),
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeProjectionRepair,
				"@every 5s",
				enrollmentasync.NewProjectionReconciliationJob(
					projectionReconciliationUsecase,
					100,
				).ProcessTask,
			),
			taskqueue.NewScheduledHandler(
				outbox.TaskTypeDispatch,
				"@every 5s",
				outboxDispatcher.ProcessTask,
			),
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeWaitlistPromotion,
				"@every 1s",
				enrollmentasync.NewWaitlistPromotionJob(waitlistUsecase, 100).ProcessTask,
			),
		},
	}
}
