package identitysecurity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type studentClaims struct {
	jwt.RegisteredClaims
}

// StudentTokenManager 统一负责学生 JWT 的签发与验证。
// 当前单体服务固定使用 HS256，签名密钥只保存在服务端。
type StudentTokenManager struct {
	signingKey []byte
	issuer     string
	audience   string
	tokenTTL   time.Duration
	clockSkew  time.Duration
	now        func() time.Time
}

func NewStudentTokenManager(
	signingKey string,
	issuer string,
	audience string,
	tokenTTL time.Duration,
	clockSkew time.Duration,
) (*StudentTokenManager, error) {
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("JWT signing key must contain at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT issuer and audience are required")
	}
	if tokenTTL <= 0 {
		return nil, fmt.Errorf("JWT token TTL must be positive")
	}
	if clockSkew < 0 {
		return nil, fmt.Errorf("JWT clock skew must not be negative")
	}
	return &StudentTokenManager{
		signingKey: []byte(signingKey),
		issuer:     strings.TrimSpace(issuer),
		audience:   strings.TrimSpace(audience),
		tokenTTL:   tokenTTL,
		clockSkew:  clockSkew,
		now:        time.Now,
	}, nil
}

func (m *StudentTokenManager) Issue(studentID uint64) (string, time.Time, error) {
	if studentID == 0 {
		return "", time.Time{}, fmt.Errorf("student ID is required")
	}
	issuedAt := m.now()
	expiresAt := issuedAt.Add(m.tokenTTL)
	claims := studentClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatUint(studentID, 10),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
		},
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign student JWT: %w", err)
	}
	return value, expiresAt, nil
}

func (m *StudentTokenManager) Verify(value string) (uint64, error) {
	claims := &studentClaims{}
	token, err := jwt.ParseWithClaims(
		strings.TrimSpace(value),
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected JWT signing method")
			}
			return m.signingKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithLeeway(m.clockSkew),
		jwt.WithTimeFunc(m.now),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return 0, fmt.Errorf("verify student JWT: %w", err)
	}
	if !token.Valid {
		return 0, fmt.Errorf("student JWT is invalid")
	}
	studentID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || studentID == 0 {
		return 0, fmt.Errorf("student JWT subject is invalid")
	}
	return studentID, nil
}
