package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type degradeConfigStub bool

func (c degradeConfigStub) IsDegraded() bool { return bool(c) }

func TestDegradeReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Degrade(degradeConfigStub(true)))
	engine.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/resource", nil))

	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.String() !=
		`{"code":503,"data":null,"info":"service degraded"}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}

func TestDegradeAllowsRequestsWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Degrade(degradeConfigStub(false)))
	engine.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/resource", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want 204", recorder.Code)
	}
}
