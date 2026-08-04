package danmakuhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	danmakuapp "github.com/yywencs/courseforge/internal/danmaku/application"
	"github.com/yywencs/courseforge/internal/danmaku/domain"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"

	"github.com/gin-gonic/gin"
)

type publisherStub struct {
	command      danmakuapp.PublishCommand
	item         *danmaku.Danmaku
	err          error
	historyQuery danmakuapp.HistoryQuery
	historyPage  *danmakuapp.HistoryPage
	historyErr   error
}

func (p *publisherStub) ListHistory(_ context.Context, query danmakuapp.HistoryQuery) (*danmakuapp.HistoryPage, error) {
	p.historyQuery = query
	return p.historyPage, p.historyErr
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
	if len(routes) != 2 || routes[0].Method != http.MethodGet ||
		routes[0].Path != "/api/v1/course-videos/:video_id/danmakus" ||
		routes[1].Method != http.MethodPost ||
		routes[1].Path != "/api/v1/course-videos/:video_id/danmakus" {
		t.Fatalf("routes = %#v", routes)
	}
}

func TestListHistoryReturnsSegmentWithoutStudentIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	segment, err := danmaku.NewHistorySegment(3)
	if err != nil {
		t.Fatal(err)
	}
	service := &publisherStub{historyPage: &danmakuapp.HistoryPage{
		Segment: segment,
		Items: []danmaku.Danmaku{{
			ID: 128, VideoID: 7, StudentID: 1001, VideoTimeMS: 125_200,
			Content: "这里讲得很清楚", Status: danmaku.StatusVisible,
		}},
	}}
	auth := middleware.NewJWTAuth(middleware.TokenVerifierFunc(func(string) (uint64, error) {
		return 1001, nil
	}))
	engine := gin.New()
	NewRoutes(service, auth).RegisterAPIRoutes(engine.Group("/api/v1"))
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/course-videos/7/danmakus?segment_index=3",
		nil,
	)
	request.Header.Set("Authorization", "Bearer student-token")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.historyQuery.VideoID != 7 || service.historyQuery.SegmentIndex != 3 {
		t.Fatalf("query = %#v", service.historyQuery)
	}
	body := response.Body.String()
	for _, want := range []string{`"segment_index":3`, `"start_ms":120000`, `"end_ms":180000`, `"video_time_ms":125200`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %s, missing %s", body, want)
		}
	}
	if strings.Contains(body, "student_id") || strings.Contains(body, "client_msg_id") {
		t.Fatalf("history response leaked sender identity: %s", body)
	}
}

func TestListHistoryRejectsMissingSegmentIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRoutes(&publisherStub{}, nil).RegisterAPIRoutes(engine.Group("/api/v1"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/course-videos/7/danmakus", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
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
