package identitysecurity

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptPasswordVerifier(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	verifier := BcryptPasswordVerifier{}

	if !verifier.Verify(string(passwordHash), "correct-password") {
		t.Fatal("Verify() rejected matching password")
	}
	if verifier.Verify(string(passwordHash), "wrong-password") {
		t.Fatal("Verify() accepted wrong password")
	}
	if verifier.Verify("", "wrong-password") {
		t.Fatal("Verify() accepted password for missing account hash")
	}
}
