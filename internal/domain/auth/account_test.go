package auth

import (
	"errors"
	"testing"
)

func TestStudentAccountValidateForLogin(t *testing.T) {
	tests := []struct {
		name    string
		account *StudentAccount
		wantErr error
	}{
		{
			name: "active account",
			account: &StudentAccount{
				ID:           10001,
				StudentNo:    "2026001001",
				PasswordHash: "bcrypt-hash",
				State:        "active",
			},
		},
		{
			name:    "missing account",
			account: nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "incomplete account",
			account: &StudentAccount{
				ID:        10001,
				StudentNo: "2026001001",
				State:     "active",
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "inactive account",
			account: &StudentAccount{
				ID:           10001,
				StudentNo:    "2026001001",
				PasswordHash: "bcrypt-hash",
				State:        "disabled",
			},
			wantErr: ErrStudentInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.account.ValidateForLogin()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateForLogin() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
