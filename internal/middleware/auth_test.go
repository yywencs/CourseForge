package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestStudentJWTAuthUsesStudentIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const key = "student-auth-test-signing-key-at-least-32-bytes"
	auth, err := NewStudentJWTAuth(
		key, "courseforge", "courseforge-student", 30*time.Second, []string{"HS256"},
	)
	if err != nil {
		t.Fatalf("NewStudentJWTAuth() error = %v", err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"student_id": "10001",
		"sub":        "10001",
		"iss":        "courseforge",
		"aud":        "courseforge-student",
		"exp":        time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	engine := gin.New()
	engine.Use(auth)
	engine.GET("/protected", func(c *gin.Context) {
		info := GetAuthInfo(c)
		c.JSON(http.StatusOK, gin.H{"student_id": info.UserID})
	})
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"student_id":"10001"}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}

func TestStudentJWTAuthRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth, err := NewStudentJWTAuth(
		"student-auth-test-signing-key-at-least-32-bytes",
		"courseforge",
		"courseforge-student",
		30*time.Second,
		nil,
	)
	if err != nil {
		t.Fatalf("NewStudentJWTAuth() error = %v", err)
	}
	engine := gin.New()
	engine.Use(auth)
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() !=
		`{"code":401,"data":null,"info":"authentication required"}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}
