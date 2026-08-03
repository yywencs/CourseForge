package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorResponseContract(t *testing.T) {
	testCases := []struct {
		name   string
		status int
	}{
		{name: "bad request", status: http.StatusBadRequest},
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "conflict", status: http.StatusConflict},
		{name: "too many requests", status: http.StatusTooManyRequests},
		{name: "internal server error", status: http.StatusInternalServerError},
		{name: "service unavailable", status: http.StatusServiceUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.GET("/error", func(c *gin.Context) {
				Error(c, testCase.status, testCase.name)
			})

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/error", nil))

			if recorder.Code != testCase.status {
				t.Fatalf("HTTP status = %d, want %d", recorder.Code, testCase.status)
			}

			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
			}
			if response.Code != testCase.status || response.Info != testCase.name || response.Data != nil {
				t.Fatalf("response = %#v, want code:%d info:%q data:nil", response, testCase.status, testCase.name)
			}
		})
	}
}
