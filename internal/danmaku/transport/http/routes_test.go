package danmakuhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	danmakuapp "prizeforge/internal/danmaku/application"
	"prizeforge/internal/danmaku/domain"
	"prizeforge/internal/platform/http/middleware"

	"github.com/gin-gonic/gin"
)

type publisherStub struct {
	command danmakuapp.PublishCommand
	item    *danmaku.Danmaku
	err     error
}

func (p *publisherStub) Publish(_ context.Context, command danmakuapp.PublishCommand) (*danmaku.Danmaku, error) {
	p.command = command
	return p.item, p.err
}

func TestRoutesRegisterPublishContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRoutes(nil, nil).RegisterAPIRoutes(engine.Group("/api/v1"))
	routes := engine.Routes()
	if len(routes) != 1 || routes[0].Method != http.MethodPost ||
		routes[0].Path != "/api/v1/course-videos/:video_id/danmakus" {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestPublishUsesAuthenticatedStudentAndReturnsPersistedDanmaku(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 8, 4, 16, 20, 31, 123000000, time.FixedZone("CST", 8*60*60))
	publisher := &publisherStub{item: &danmaku.Danmaku{
		ID: 128, VideoID: 7, StudentID: 1001,
		ClientMessageID: "ec40a0ec-572c-4af5-9067-65f702fa666c",
		VideoTimeMS:     65200, Content: "这里讲得很清楚",
		Status: danmaku.StatusVisible, CreateTime: now,
	}}
	auth := middleware.NewJWTAuth(middleware.TokenVerifierFunc(func(string) (uint64, error) {
		return 1001, nil
	}))
	engine := gin.New()
	NewRoutes(publisher, auth).RegisterAPIRoutes(engine.Group("/api/v1"))

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/course-videos/7/danmakus",
		strings.NewReader(`{"client_msg_id":"ec40a0ec-572c-4af5-9067-65f702fa666c","video_time_ms":65200,"content":"这里讲得很清楚"}`),
	)
	request.Header.Set("Authorization", "Bearer student-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if publisher.command.StudentID != 1001 || publisher.command.VideoID != 7 ||
		publisher.command.VideoTimeMS != 65200 {
		t.Fatalf("command = %#v", publisher.command)
	}
	for _, want := range []string{`"id":128`, `"student_id":1001`, `"status":"visible"`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, missing %s", response.Body.String(), want)
		}
	}
}

func TestPublishRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	auth := middleware.NewJWTAuth(middleware.TokenVerifierFunc(func(string) (uint64, error) {
		return 1001, nil
	}))
	NewRoutes(&publisherStub{}, auth).RegisterAPIRoutes(engine.Group("/api/v1"))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/course-videos/7/danmakus", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
