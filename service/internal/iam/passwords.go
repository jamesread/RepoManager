package iam

import (
	"strings"

	armpassword "github.com/jamesread/armature-iam/password"
	"golang.org/x/crypto/bcrypt"
)

// VerifyPassword checks argon2id (armature-iam) and legacy bcrypt (migrated SickRock users).
func VerifyPassword(storedHash, password string) (bool, error) {
	if storedHash == "" {
		return false, nil
	}
	if strings.HasPrefix(storedHash, "$argon2") {
		return armpassword.Verify(storedHash, password)
	}
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") || strings.HasPrefix(storedHash, "$2y$") {
		err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
		return err == nil, nil
	}
	return armpassword.Verify(storedHash, password)
}

// HashPassword stores new passwords with argon2id.
func HashPassword(password string) (string, error) {
	return armpassword.Hash(password)
}
