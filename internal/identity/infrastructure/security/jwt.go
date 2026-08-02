package identitysecurity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	studentActorType       = "student"
	administratorActorType = "administrator"
)

type identityClaims struct {
	ActorType string `json:"actor_type"`
	jwt.RegisteredClaims
}

// TokenManager 复用同一套 JWT 实现与注册声明管理学生和管理员令牌。
// actor_type 是两类身份的强制边界，避免同一个 subject 被跨端解释。
type TokenManager struct {
	signingKey []byte
	issuer     string
	audience   string
	tokenTTL   time.Duration
	clockSkew  time.Duration
	now        func() time.Time
}

// StudentTokenManager 保留原类型名，兼容现有学生认证装配代码。
type StudentTokenManager = TokenManager

func NewTokenManager(
	signingKey string,
	issuer string,
	audience string,
	tokenTTL time.Duration,
	clockSkew time.Duration,
) (*TokenManager, error) {
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
	return &TokenManager{
		signingKey: []byte(signingKey),
		issuer:     strings.TrimSpace(issuer),
		audience:   strings.TrimSpace(audience),
		tokenTTL:   tokenTTL,
		clockSkew:  clockSkew,
		now:        time.Now,
	}, nil
}

func NewStudentTokenManager(
	signingKey string,
	issuer string,
	audience string,
	tokenTTL time.Duration,
	clockSkew time.Duration,
) (*StudentTokenManager, error) {
	return NewTokenManager(signingKey, issuer, audience, tokenTTL, clockSkew)
}

func (m *TokenManager) Issue(studentID uint64) (string, time.Time, error) {
	if studentID == 0 {
		return "", time.Time{}, fmt.Errorf("student ID is required")
	}
	return m.issue(studentID, studentActorType)
}

func (m *TokenManager) IssueAdministrator(
	administratorID uint64,
) (string, time.Time, error) {
	if administratorID == 0 {
		return "", time.Time{}, fmt.Errorf("administrator ID is required")
	}
	return m.issue(administratorID, administratorActorType)
}

func (m *TokenManager) issue(
	identityID uint64,
	actorType string,
) (string, time.Time, error) {
	issuedAt := m.now()
	expiresAt := issuedAt.Add(m.tokenTTL)
	claims := identityClaims{
		ActorType: actorType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatUint(identityID, 10),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
		},
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign %s JWT: %w", actorType, err)
	}
	return value, expiresAt, nil
}

func (m *TokenManager) Verify(value string) (uint64, error) {
	return m.verify(value, studentActorType)
}

func (m *TokenManager) VerifyAdministrator(value string) (uint64, error) {
	return m.verify(value, administratorActorType)
}

func (m *TokenManager) verify(value string, expectedActorType string) (uint64, error) {
	claims := &identityClaims{}
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
		return 0, fmt.Errorf("verify %s JWT: %w", expectedActorType, err)
	}
	if !token.Valid {
		return 0, fmt.Errorf("%s JWT is invalid", expectedActorType)
	}
	if claims.ActorType != expectedActorType {
		return 0, fmt.Errorf("JWT actor type is invalid")
	}
	identityID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || identityID == 0 {
		return 0, fmt.Errorf("%s JWT subject is invalid", expectedActorType)
	}
	return identityID, nil
}
