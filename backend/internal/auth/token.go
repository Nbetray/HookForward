package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenIssuer struct {
	secret []byte
}

var ErrInvalidToken = errors.New("invalid token")

type UserClaims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret)}
}

func (i *TokenIssuer) Issue(userID string, email string, role string) (string, time.Time, error) {
	expiresAt := time.Now().UTC().Add(12 * time.Hour)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		UserID: userID,
		Role:   role,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	})

	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (i *TokenIssuer) Parse(tokenString string) (UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return i.secret, nil
	})
	if err != nil {
		return UserClaims{}, err
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return UserClaims{}, ErrInvalidToken
	}

	return *claims, nil
}
