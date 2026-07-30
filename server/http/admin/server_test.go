package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

func TestServerRegistersHealthRoutes(t *testing.T) {
	server := NewServer(":0", common.ReadinessChecks{
		"dependency": func(context.Context) error { return nil },
	})

	for _, path := range []string{"/healthz", "/readyz"} {
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
