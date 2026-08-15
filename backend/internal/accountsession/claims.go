package accountsession

import (
	"github.com/golang-jwt/jwt/v5"
)

const TokenTypeAccess = "access"

type AccessClaims struct {
	UserID    int    `json:"userId"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func (c AccessClaims) GetSessionID() string {
	return c.SessionID
}

func (c AccessClaims) GetUserID() int {
	return c.UserID
}
