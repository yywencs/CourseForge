package enrollmentobservability

import (
	"time"

	application "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
)

// PrometheusObserver translates application outcomes to the existing metric
// names and labels without exposing Prometheus to the application layer.
type PrometheusObserver struct{}

func NewPrometheusObserver() PrometheusObserver {
	return PrometheusObserver{}
}

func (PrometheusObserver) SelectionCompleted(
	outcome application.SelectionOutcome,
	duration time.Duration,
) {
	metrics.ObserveSelection(string(outcome), duration)
}

func (PrometheusObserver) ProjectionUpdated(
	operation application.ProjectionOperation,
	outcome application.ProjectionOutcome,
) {
	metrics.IncEnrollmentProjection(string(operation), string(outcome))
}

func (PrometheusObserver) WaitlistPromotionCompleted(
	outcome application.WaitlistPromotionOutcome,
) {
	metrics.IncWaitlistPromotion(string(outcome))
}

func (PrometheusObserver) ProjectionRepairBacklogObserved(count int64) {
	metrics.SetProjectionRepairPending(count)
}

var _ application.EnrollmentObserver = PrometheusObserver{}
