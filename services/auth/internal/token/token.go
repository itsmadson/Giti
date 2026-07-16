// Package token issues and verifies Geoson JWTs (HS256).
package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func Issue(secret []byte, username string, roles []string, ttl time.Duration) (string, error) {
	c := claims{
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "geoson",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
}

func Verify(secret []byte, tok string) (string, []string, error) {
	var c claims
	t, err := jwt.ParseWithClaims(tok, &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !t.Valid {
		return "", nil, errors.New("invalid token")
	}
	return c.Subject, c.Roles, nil
}
