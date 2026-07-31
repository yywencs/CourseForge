package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := os.Getenv("COURSEFORGE_ACCOUNT_PASSWORD")
	if password == "" {
		log.Fatal("COURSEFORGE_ACCOUNT_PASSWORD is required")
	}
	if len(password) < 8 {
		log.Fatal("password must contain at least 8 bytes")
	}
	if len(password) > 72 {
		log.Fatal("password must not exceed 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	fmt.Println(string(hash))
}
