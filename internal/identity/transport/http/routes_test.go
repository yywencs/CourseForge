package identityhttp

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIdentityRoutesPreservePublicAndProtectedContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	auth := func(c *gin.Context) { c.Next() }
	NewRoutes(nil, auth).RegisterAPIRoutes(engine.Group("/api/v1"))

	want := map[string]struct{}{
		http.MethodPost + " /api/v1/auth/login": {},
		http.MethodGet + " /api/v1/auth/me":     {},
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing identity routes: %v", want)
	}
}
