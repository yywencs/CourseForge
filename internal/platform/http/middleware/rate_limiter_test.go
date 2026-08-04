package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/config"

	"github.com/gin-gonic/gin"
)

func TestLoginRateLimiterKeepsAccountBucketsIndependent(t *testing.T) {
	cfg := testRateLimitConfig()
	cfg.Login.Account = testPolicy(1, time.Minute, 1)
	limiter := NewLoginRateLimiter(cfg)
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("192.0.2.1", "student-a") {
		t.Fatal("first account request was rejected")
	}
	if !limiter.Allow("192.0.2.1", "student-b") {
		t.Fatal("different account sharing the same IP was rejected")
	}
	if limiter.Allow("192.0.2.1", "student-a") {
		t.Fatal("exhausted account bucket accepted another request")
	}
}

func TestLoginRateLimiterStopsRotatingAccountsByIP(t *testing.T) {
	cfg := testRateLimitConfig()
	cfg.Login.IP = testPolicy(2, time.Minute, 2)
	limiter := NewLoginRateLimiter(cfg)
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("192.0.2.1", "student-a") ||
		!limiter.Allow("192.0.2.1", "student-b") {
		t.Fatal("requests within the IP burst were rejected")
	}
	if limiter.Allow("192.0.2.1", "student-c") {
		t.Fatal("rotating accounts bypassed the exhausted IP bucket")
	}
}

func TestRateLimitersCanBeDisabled(t *testing.T) {
	cfg := testRateLimitConfig()
	cfg.Enabled = false
	login := NewLoginRateLimiter(cfg)
	selection := NewSelectionRateLimiter(cfg)

	for range 100 {
		if !login.Allow("192.0.2.1", "student-a") {
			t.Fatal("disabled login limiter rejected a request")
		}
		if !selection.Allow(10001) {
			t.Fatal("disabled selection limiter rejected a request")
		}
	}
}

func TestLocalKeyedLimiterExpiresIdleEntries(t *testing.T) {
	limiter := newLocalKeyedLimiter(
		testPolicy(1, time.Minute, 1),
		time.Minute,
		1,
	)
	start := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	if !limiter.allowAt("student-a", start) {
		t.Fatal("first key was rejected")
	}
	if !limiter.allowAt("student-b", start.Add(2*time.Minute)) {
		t.Fatal("expired key was not removed before admitting a new key")
	}
}

func TestLocalKeyedLimiterRejectsNewKeysAtCapacity(t *testing.T) {
	limiter := newLocalKeyedLimiter(
		testPolicy(10, time.Second, 10),
		10*time.Minute,
		1,
	)
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	if !limiter.allowAt("student-a", now) {
		t.Fatal("first key was rejected")
	}
	if limiter.allowAt("student-b", now) {
		t.Fatal("new key was admitted after reaching the entry cap")
	}
}

func TestLocalKeyedLimiterIsSafeUnderConcurrency(t *testing.T) {
	const burst = 10
	limiter := newLocalKeyedLimiter(
		testPolicy(burst, time.Minute, burst),
		10*time.Minute,
		100,
	)
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	var accepted atomic.Int64
	var group sync.WaitGroup
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			if limiter.allowAt("student-a", now) {
				accepted.Add(1)
			}
		}()
	}
	group.Wait()
	if got := accepted.Load(); got != burst {
		t.Fatalf("accepted requests = %d, want %d", got, burst)
	}
}

func TestSelectionRateLimiterUsesAuthenticatedStudent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testRateLimitConfig()
	cfg.Selection.Student = testPolicy(1, time.Minute, 1)
	limiter := NewSelectionRateLimiter(cfg)
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	engine := gin.New()
	engine.POST("/enrollments", func(c *gin.Context) {
		c.Set(authenticatedSubjectIDKey, uint64(10001))
		c.Next()
	}, limiter.Handle, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	engine.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/enrollments", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first response status = %d, want 204", first.Code)
	}

	second := httptest.NewRecorder()
	engine.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/enrollments", nil))
	if second.Code != http.StatusTooManyRequests ||
		!strings.Contains(second.Body.String(), `"code":429`) {
		t.Fatalf("limited response = status:%d body:%s", second.Code, second.Body.String())
	}
}

func testRateLimitConfig() config.RateLimitConfig {
	return config.RateLimitConfig{
		Enabled: true, EntryTTL: 10 * time.Minute, MaxEntries: 100,
		Login: config.LoginRateLimitConfig{
			Global:  testPolicy(1000, time.Second, 1000),
			IP:      testPolicy(1000, time.Second, 1000),
			Account: testPolicy(1000, time.Second, 1000),
		},
		Selection: config.SelectionRateLimitConfig{
			Global:  testPolicy(1000, time.Second, 1000),
			Student: testPolicy(1000, time.Second, 1000),
		},
	}
}

func testPolicy(
	requests int,
	window time.Duration,
	burst int,
) config.RateLimitPolicyConfig {
	return config.RateLimitPolicyConfig{
		Requests: requests,
		Window:   window,
		Burst:    burst,
	}
}
