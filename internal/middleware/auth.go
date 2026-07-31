package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const authenticatedStudentIDKey = "authenticatedStudentID"

type StudentTokenVerifier interface {
	Verify(token string) (uint64, error)
}

// NewStudentJWTAuth 只接受 Authorization: Bearer JWT，并将验签后的学生 ID
// 写入 Gin Context。Token 的签发与密码校验不属于 HTTP 中间件职责。
func NewStudentJWTAuth(verifier StudentTokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || verifier == nil {
			abortAuthentication(c)
			return
		}
		studentID, err := verifier.Verify(token)
		if err != nil || studentID == 0 {
			abortAuthentication(c)
			return
		}
		c.Set(authenticatedStudentIDKey, studentID)
		c.Next()
	}
}

func AuthenticatedStudentID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(authenticatedStudentIDKey)
	if !ok {
		return 0, false
	}
	studentID, ok := value.(uint64)
	return studentID, ok && studentID > 0
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	token = strings.TrimSpace(token)
	return token, found && strings.EqualFold(scheme, "Bearer") && token != ""
}

func abortAuthentication(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{
		"code": 401,
		"info": "authentication required",
		"data": nil,
	})
}
