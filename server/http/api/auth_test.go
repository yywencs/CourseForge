package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationapi "prizeforge/internal/application/api"
	authdomain "prizeforge/internal/domain/auth"

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
		StudentNo:    "2026001001",
		StudentName:  "林知夏",
		PasswordHash: r.passwordHash,
		State:        "active",
	}, nil
}

func (handlerAuthRepository) FindStudentByID(
	context.Context,
	uint64,
) (*authdomain.StudentAccount, error) {
	return nil, authdomain.ErrAccountNotFound
}

func (handlerAuthRepository) FindCurrentSelectionContext(
	context.Context,
) (*authdomain.SelectionContext, error) {
	return &authdomain.SelectionContext{TermID: 1, RoundID: 2}, nil
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
		handlerAuthRepository{passwordHash: string(passwordHash)},
		handlerTokenIssuer{},
	)
	server := NewServer(":0", usecase, nil, nil, nil, nil, nil)
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
