package accountsession

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
)

type TokenService struct {
	secret     []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
	absoluteTTL time.Duration
}

func NewTokenService() *TokenService {
	cfg := config.AppCfg
	accessTTL := time.Duration(cfg.JWT.AccessTokenMinutes) * time.Minute
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := time.Duration(cfg.JWT.RefreshTokenDays) * 24 * time.Hour
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}
	absoluteTTL := time.Duration(cfg.JWT.AbsoluteSessionDays) * 24 * time.Hour
	if absoluteTTL <= 0 {
		absoluteTTL = 90 * 24 * time.Hour
	}
	return &TokenService{
		secret:      []byte(cfg.JWT.Secret),
		issuer:      cfg.JWT.Issuer,
		audience:    cfg.JWT.Audience,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		absoluteTTL: absoluteTTL,
	}
}

func NewTokenServiceFromParams(secret, issuer, audience string) *TokenService {
	return &TokenService{
		secret:    []byte(secret),
		issuer:    issuer,
		audience:  audience,
		accessTTL: 15 * time.Minute,
	}
}

func (s *TokenService) SignAccessToken(userID int, username, role, sessionID string) (string, time.Time, error) {
	jti := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(s.accessTTL)
	claims := AccessClaims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		SessionID: sessionID,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (s *TokenService) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
	)
	if err != nil {
		return nil, ErrAccessInvalid
	}
	if !token.Valid {
		return nil, ErrAccessInvalid
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrAccessInvalid
	}
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrAccessExpired
	}
	return claims, nil
}

func GenerateOpaqueToken(prefix string) (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func GeneratePublicID(prefix string) string {
	return prefix + uuid.NewString()
}

func (s *TokenService) AccessTTL() time.Duration {
	return s.accessTTL
}

func (s *TokenService) RefreshTTL() time.Duration {
	return s.refreshTTL
}

func (s *TokenService) AbsoluteTTL() time.Duration {
	return s.absoluteTTL
}
