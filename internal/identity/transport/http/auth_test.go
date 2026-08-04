package identityhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationapi "github.com/yywencs/courseforge/internal/identity/application"
	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
	authinfra "github.com/yywencs/courseforge/internal/identity/infrastructure/security"
	apihttp "github.com/yywencs/courseforge/server/http/api"

	"golang.org/x/crypto/bcrypt"
)

type handlerAuthRepository struct {
	passwordHash string
}

func (r handlerAuthRepository) FindStudentByNumber(
	context.Context,
	string,
) (*authdomain.StudentAccount, error) {
	return &authdomain.StudentAccount{
		ID:           10001,
		AccountID:    20001,
		StudentNo:    "2026001001",
		StudentName:  "林知夏",
		PasswordHash: r.passwordHash,
		AccountState: "enabled",
	}, nil
}

func (handlerAuthRepository) FindStudentByID(
	context.Context,
	uint64,
) (*authdomain.StudentAccount, error) {
	return nil, authdomain.ErrAccountNotFound
}

type handlerSelectionContextQuery struct{}

func (handlerSelectionContextQuery) FindCurrentSelectionContext(
	context.Context,
) (*applicationapi.SelectionContext, error) {
	return &applicationapi.SelectionContext{TermID: 1, RoundID: 2}, nil
}

type handlerTokenIssuer struct{}

func (handlerTokenIssuer) Issue(uint64) (string, time.Time, error) {
	return "signed-token", time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC), nil
}

func TestLoginReturnsStudentSessionWithoutPasswordHash(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	usecase := applicationapi.NewAuthenticationUsecase(
		authdomain.NewAuthenticator(
			handlerAuthRepository{passwordHash: string(passwordHash)},
			authinfra.BcryptPasswordVerifier{},
		),
		handlerSelectionContextQuery{},
		handlerTokenIssuer{},
	)
	server := apihttp.NewServer(":0", nil, NewRoutes(usecase, nil))
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		bytes.NewBufferString(`{"student_no":"2026001001","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"access_token":"signed-token"`) ||
		!strings.Contains(recorder.Body.String(), `"student_no":"2026001001"`) {
		t.Fatalf("response = status:%d body:%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), string(passwordHash)) {
		t.Fatalf("response leaked password hash: %s", recorder.Body.String())
	}
}

func TestStudentLoginAppliesRateLimiter(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	usecase := applicationapi.NewAuthenticationUsecase(
		authdomain.NewAuthenticator(
			handlerAuthRepository{passwordHash: string(passwordHash)},
			authinfra.BcryptPasswordVerifier{},
		),
		handlerSelectionContextQuery{},
		handlerTokenIssuer{},
	)
	server := apihttp.NewServer(
		":0",
		nil,
		NewRoutes(usecase, nil, accountRejectingLoginLimiter{}),
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		bytes.NewBufferString(`{"student_no":"2026001001","password":"correct-password"}`),
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
