package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var ErrWeakPassword = errors.New("password must be at least 8 characters and contain a letter and a digit")

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, ch := range password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

func HashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func VerifyPassword(hashed string, raw string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(raw))
}
