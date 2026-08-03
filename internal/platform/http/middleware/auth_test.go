package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubTokenVerifier struct {
	subjectID uint64
	err       error
}

func (s stubTokenVerifier) Verify(string) (uint64, error) {
	return s.subjectID, s.err
}

func TestJWTAuthUsesVerifiedSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assertAuthenticationAccepted(t, stubTokenVerifier{subjectID: 10001})
}

func TestJWTAuthAcceptsFunctionVerifier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assertAuthenticationAccepted(t, TokenVerifierFunc(func(string) (uint64, error) {
		return 10001, nil
	}))
}

func TestJWTAuthRejectsMissingOrInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name     string
		header   string
		verifier TokenVerifier
	}{
		{name: "missing token", verifier: stubTokenVerifier{subjectID: 10001}},
		{name: "wrong scheme", header: "Basic token", verifier: stubTokenVerifier{subjectID: 10001}},
		{name: "invalid token", header: "Bearer invalid", verifier: stubTokenVerifier{err: fmt.Errorf("invalid")}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertAuthenticationRejected(t, testCase.header, testCase.verifier)
		})
	}
}

func TestJWTAuthFunctionVerifierRejectsMissingOrInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name   string
		header string
		verify TokenVerifierFunc
	}{
		{name: "missing token", verify: func(string) (uint64, error) { return 10001, nil }},
		{name: "wrong scheme", header: "Basic token", verify: func(string) (uint64, error) { return 10001, nil }},
		{name: "invalid token", header: "Bearer invalid", verify: func(string) (uint64, error) { return 0, fmt.Errorf("invalid") }},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertAuthenticationRejected(t, testCase.header, testCase.verify)
		})
	}
}

func assertAuthenticationAccepted(t *testing.T, verifier TokenVerifier) {
	t.Helper()
	engine := gin.New()
	engine.Use(NewJWTAuth(verifier))
	engine.GET("/protected", func(c *gin.Context) {
		subjectID, ok := AuthenticatedSubjectID(c)
		c.JSON(http.StatusOK, gin.H{"subject_id": subjectID, "ok": ok})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK ||
		recorder.Body.String() != `{"ok":true,"subject_id":10001}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}

func assertAuthenticationRejected(t *testing.T, header string, verifier TokenVerifier) {
	t.Helper()
	engine := gin.New()
	engine.Use(NewJWTAuth(verifier))
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", header)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || recorder.Body.String() !=
		`{"code":401,"data":null,"info":"authentication required"}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}
