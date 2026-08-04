package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformmiddleware "github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
)

func TestServerRegistersManagementRoutes(t *testing.T) {
	server := NewServer(":0", common.ReadinessChecks{
		"dependency": func(context.Context) error { return nil },
	})

	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		server.Engine().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

func TestServerProvidesStatusAndExtensionRoutes(t *testing.T) {
	server := NewServer(":0", nil, testRouteRegistrar{})

	for _, path := range []string{"/admin/v1/status", "/admin/v1/courses"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		server.Engine().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}
}

type testRouteRegistrar struct{}

func (testRouteRegistrar) RegisterAdminRoutes(group *gin.RouterGroup) {
	group.GET("/courses", func(ctx *gin.Context) {
		common.Success(ctx, gin.H{"items": []any{}})
	})
}

type authenticatedTestRouteRegistrar struct{}

func (authenticatedTestRouteRegistrar) RegisterPublicAdminRoutes(group *gin.RouterGroup) {
	group.POST("/auth/login", func(ctx *gin.Context) {
		common.Success(ctx, gin.H{"access_token": "administrator-token"})
	})
}

func (authenticatedTestRouteRegistrar) RegisterAdminRoutes(group *gin.RouterGroup) {
	group.GET("/courses", func(ctx *gin.Context) {
		common.Success(ctx, gin.H{"items": []any{}})
	})
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (uint64, error) {
	if token == "administrator-token" {
		return 30001, nil
	}
	return 0, context.Canceled
}

func TestAuthenticatedServerKeepsLoginPublicAndProtectsManagementRoutes(t *testing.T) {
	server := NewAuthenticatedServer(
		":0",
		nil,
		platformmiddleware.NewJWTAuth(tokenVerifierStub{}),
		authenticatedTestRouteRegistrar{},
	)

	for _, testCase := range []struct {
		method        string
		path          string
		authorization string
		wantStatus    int
		wantBodyPart  string
	}{
		{method: http.MethodGet, path: "/admin/v1/status", wantStatus: http.StatusOK, wantBodyPart: `"code":0`},
		{method: http.MethodPost, path: "/admin/v1/auth/login", wantStatus: http.StatusOK, wantBodyPart: `"code":0`},
		{method: http.MethodGet, path: "/admin/v1/courses", wantStatus: http.StatusUnauthorized, wantBodyPart: `"code":401`},
		{
			method: http.MethodGet, path: "/admin/v1/courses",
			authorization: "Bearer administrator-token", wantStatus: http.StatusOK, wantBodyPart: `"code":0`,
		},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		request.Header.Set("Authorization", testCase.authorization)
		server.Engine().ServeHTTP(response, request)
		if response.Code != testCase.wantStatus ||
			!strings.Contains(response.Body.String(), testCase.wantBodyPart) {
			t.Errorf("%s %s response = status:%d body:%s", testCase.method, testCase.path, response.Code, response.Body.String())
		}
	}
}
