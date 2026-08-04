package enrollmentapp

import (
	"os"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/observability/logger"

	"go.uber.org/zap"
)

type fixedIDGenerator struct {
	id  string
	err error
}

func (g fixedIDGenerator) NewID() (string, error) {
	return g.id, g.err
}

type noopEnrollmentObserver struct{}

func (noopEnrollmentObserver) SelectionCompleted(SelectionOutcome, time.Duration)       {}
func (noopEnrollmentObserver) ProjectionUpdated(ProjectionOperation, ProjectionOutcome) {}
func (noopEnrollmentObserver) WaitlistPromotionCompleted(WaitlistPromotionOutcome)      {}
func (noopEnrollmentObserver) ProjectionRepairBacklogObserved(int64)                    {}

// TestMain 为 API 应用层测试安装无输出 Logger，避免测试依赖生产日志文件配置。
func TestMain(m *testing.M) {
	logger.Log = zap.NewNop()
	zap.ReplaceGlobals(logger.Log)
	os.Exit(m.Run())
}
