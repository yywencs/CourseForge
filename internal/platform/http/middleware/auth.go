package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const authenticatedSubjectIDKey = "authenticatedSubjectID"

// TokenVerifier 将令牌验证为一个与具体业务身份无关的 subject ID。
type TokenVerifier interface {
	Verify(token string) (uint64, error)
}

// TokenVerifierFunc 让不同身份的验签方法可以适配为通用 TokenVerifier。
type TokenVerifierFunc func(token string) (uint64, error)

func (f TokenVerifierFunc) Verify(token string) (uint64, error) {
	return f(token)
}

// NewJWTAuth 只接受 Authorization: Bearer JWT，并将验签后的 subject ID
// 写入 Gin Context。具体身份类型及令牌声明由调用方提供的 verifier 校验。
func NewJWTAuth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || verifier == nil {
			abortAuthentication(c)
			return
		}
		subjectID, err := verifier.Verify(token)
		if err != nil || subjectID == 0 {
			abortAuthentication(c)
			return
		}
		c.Set(authenticatedSubjectIDKey, subjectID)
		c.Next()
	}
}

// AuthenticatedSubjectID 返回当前请求已经验签的 subject ID。
func AuthenticatedSubjectID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(authenticatedSubjectIDKey)
	if !ok {
		return 0, false
	}
	subjectID, ok := value.(uint64)
	return subjectID, ok && subjectID > 0
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	token = strings.TrimSpace(token)
	return token, found && strings.EqualFold(scheme, "Bearer") && token != ""
}

func abortAuthentication(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code": http.StatusUnauthorized,
		"info": "authentication required",
		"data": nil,
	})
}
