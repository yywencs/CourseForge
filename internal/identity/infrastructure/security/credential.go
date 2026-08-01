package identitysecurity

import (
	"strings"

	authdomain "prizeforge/internal/identity/domain"

	"golang.org/x/crypto/bcrypt"
)

const dummyPasswordHash = "$2a$10$0Cfn6AvbZhWjmDwjqyPnpuMlW.1X7RVnKB.TtRNDsfFd7NJ0xWO3e"

type BcryptPasswordVerifier struct{}

var _ authdomain.PasswordVerifier = BcryptPasswordVerifier{}

func (BcryptPasswordVerifier) Verify(passwordHash string, password string) bool {
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		passwordHash = dummyPasswordHash
	}
	return bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	) == nil
}
