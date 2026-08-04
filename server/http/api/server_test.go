package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yywencs/courseforge/server/http/common"
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
