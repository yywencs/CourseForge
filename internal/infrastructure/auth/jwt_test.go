package authinfra

import (
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestStudentTokenManagerIssuesAndVerifiesToken(t *testing.T) {
	manager := newTestStudentTokenManager(t)
	fixedNow := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return fixedNow }

	token, expiresAt, err := manager.Issue(10001)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !expiresAt.Equal(fixedNow.Add(2 * time.Hour)) {
		t.Fatalf("ExpiresAt = %v", expiresAt)
	}
	studentID, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if studentID != 10001 {
		t.Fatalf("student ID = %d, want 10001", studentID)
	}
}

func TestStudentTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := newTestStudentTokenManager(t)
	fixedNow := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return fixedNow }
	token, _, err := manager.Issue(10001)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	manager.now = func() time.Time { return fixedNow.Add(3 * time.Hour) }
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("Verify() accepted expired token")
	}
}

func TestStudentTokenManagerRequiresExpiration(t *testing.T) {
	manager := newTestStudentTokenManager(t)
	fixedNow := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return fixedNow }
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:   manager.issuer,
		Subject:  strconv.FormatUint(10001, 10),
		Audience: jwt.ClaimStrings{manager.audience},
		IssuedAt: jwt.NewNumericDate(fixedNow),
	}).SignedString(manager.signingKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := manager.Verify(value); err == nil {
		t.Fatal("Verify() accepted token without expiration")
	}
}

func newTestStudentTokenManager(t *testing.T) *StudentTokenManager {
	t.Helper()
	manager, err := NewStudentTokenManager(
		"student-auth-test-signing-key-at-least-32-bytes",
		"courseforge",
		"courseforge-student",
		2*time.Hour,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("NewStudentTokenManager() error = %v", err)
	}
	return manager
}
