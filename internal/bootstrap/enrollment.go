package bootstrap

import (
	"time"

	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	enrollmentasync "github.com/yywencs/courseforge/internal/enrollment/async"
	enrollmentobservability "github.com/yywencs/courseforge/internal/enrollment/infrastructure/observability"
	enrollmentrepo "github.com/yywencs/courseforge/internal/enrollment/infrastructure/persistence"
	enrollmenthttp "github.com/yywencs/courseforge/internal/enrollment/transport/http"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/internal/platform/identifier"
	"github.com/yywencs/courseforge/internal/platform/outbox"
	outboxdispatcher "github.com/yywencs/courseforge/internal/platform/outbox/dispatcher"
	outboxrepo "github.com/yywencs/courseforge/internal/platform/outbox/mysql"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"

	"github.com/gin-gonic/gin"
)

type enrollmentModule struct {
	routes            *enrollmenthttp.Routes
	scheduledHandlers []taskqueue.ScheduledHandler
	eligibilityIndex  *enrollmentrepo.EligibilityIndex
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
		stores.EligibilityIndex,
	)
	roundWarmupService := enrollmentapp.NewRoundWarmupService(
		stores.Eligibility,
		stores.EligibilityIndex,
		ids,
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
	countProjectionUsecase := enrollmentapp.NewEnrollmentCountProjectionUsecase(
		stores.CountProjections,
		7*24*time.Hour,
	)

	selectionLimiter := middleware.NewSelectionRateLimiter(runtime.cfg.Dcc.RateLimit)
	return &enrollmentModule{
		eligibilityIndex: stores.EligibilityIndex,
		routes: enrollmenthttp.NewRoutes(
			enrollmentUsecase,
			dropEnrollmentUsecase,
			waitlistUsecase,
			authMiddleware,
			selectionLimiter.Handle,
		),
		scheduledHandlers: []taskqueue.ScheduledHandler{
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeRoundWarmup,
				"",
				enrollmentasync.NewRoundWarmupJob(roundWarmupService).ProcessTask,
			),
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeEnrollmentCountProjection,
				"@every 1s",
				enrollmentasync.NewEnrollmentCountProjectionJob(
					countProjectionUsecase,
					500,
				).ProcessTask,
			),
			taskqueue.NewScheduledHandler(
				enrollmentasync.TaskTypeEnrollmentCountCleanup,
				"@every 1h",
				enrollmentasync.NewEnrollmentCountCleanupJob(
					countProjectionUsecase,
					1000,
				).ProcessTask,
			),
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
