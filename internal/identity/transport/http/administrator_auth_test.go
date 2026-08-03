package identityhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "prizeforge/internal/identity/application"
	authdomain "prizeforge/internal/identity/domain"
	authinfra "prizeforge/internal/identity/infrastructure/security"
	adminhttp "prizeforge/server/http/admin"

	"golang.org/x/crypto/bcrypt"
)

type handlerAdministratorRepository struct {
	passwordHash string
}

func (r handlerAdministratorRepository) FindAdministratorByUsername(
	context.Context,
	string,
) (*authdomain.AdministratorAccount, error) {
	return &authdomain.AdministratorAccount{
		ID: 30001, AccountID: 30002, Username: "admin",
		PasswordHash: r.passwordHash, AccountState: authdomain.AccountStateEnabled,
	}, nil
}

func (handlerAdministratorRepository) FindAdministratorByID(
	context.Context,
	uint64,
) (*authdomain.AdministratorAccount, error) {
	return nil, authdomain.ErrAdministratorNotFound
}

type handlerAdministratorTokenIssuer struct{}

func (handlerAdministratorTokenIssuer) IssueAdministrator(
	uint64,
) (string, time.Time, error) {
	return "administrator-token", time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC), nil
}

func TestAdministratorLoginReturnsSessionWithoutPasswordHash(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	usecase := identityapp.NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(
			handlerAdministratorRepository{passwordHash: string(passwordHash)},
			authinfra.BcryptPasswordVerifier{},
		),
		handlerAdministratorTokenIssuer{},
	)
	server := adminhttp.NewServer(":0", nil, NewAdministratorRoutes(usecase))
	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"access_token":"administrator-token"`) ||
		!strings.Contains(recorder.Body.String(), `"username":"admin"`) {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), string(passwordHash)) {
		t.Fatalf("response leaked password hash: %s", recorder.Body.String())
	}
}

func TestAdministratorLoginRejectsWrongPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	usecase := identityapp.NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(
			handlerAdministratorRepository{passwordHash: string(passwordHash)},
			authinfra.BcryptPasswordVerifier{},
		),
		handlerAdministratorTokenIssuer{},
	)
	server := adminhttp.NewServer(":0", nil, NewAdministratorRoutes(usecase))
	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"wrong-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized ||
		!strings.Contains(recorder.Body.String(), `"code":401`) ||
		!strings.Contains(recorder.Body.String(), `"info":"用户名或密码错误"`) {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdministratorLoginAppliesRateLimiter(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	usecase := identityapp.NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(
			handlerAdministratorRepository{passwordHash: string(passwordHash)},
			authinfra.BcryptPasswordVerifier{},
		),
		handlerAdministratorTokenIssuer{},
	)
	server := adminhttp.NewServer(
		":0",
		nil,
		NewAdministratorRoutes(usecase, sourceRejectingLoginLimiter{}),
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/admin/v1/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests ||
		!strings.Contains(recorder.Body.String(), `"code":429`) ||
		strings.Contains(recorder.Body.String(), `"access_token"`) {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
}
