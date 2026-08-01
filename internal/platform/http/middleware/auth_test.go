package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubStudentTokenVerifier struct {
	studentID uint64
	err       error
}

func (s stubStudentTokenVerifier) Verify(string) (uint64, error) {
	return s.studentID, s.err
}

func TestStudentJWTAuthUsesVerifiedStudentIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(NewStudentJWTAuth(stubStudentTokenVerifier{studentID: 10001}))
	engine.GET("/protected", func(c *gin.Context) {
		studentID, ok := AuthenticatedStudentID(c)
		c.JSON(http.StatusOK, gin.H{"student_id": studentID, "ok": ok})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK ||
		recorder.Body.String() != `{"ok":true,"student_id":10001}` {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}

func TestStudentJWTAuthRejectsMissingOrInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name     string
		header   string
		verifier StudentTokenVerifier
	}{
		{name: "missing token", verifier: stubStudentTokenVerifier{studentID: 10001}},
		{name: "wrong scheme", header: "Basic abc", verifier: stubStudentTokenVerifier{studentID: 10001}},
		{name: "invalid signature", header: "Bearer invalid", verifier: stubStudentTokenVerifier{err: fmt.Errorf("invalid")}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(NewStudentJWTAuth(testCase.verifier))
			engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", testCase.header)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK || recorder.Body.String() !=
				`{"code":401,"data":null,"info":"authentication required"}` {
				t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
