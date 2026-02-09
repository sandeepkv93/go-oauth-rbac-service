package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	TokenType   string   `json:"token_type"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	issuer        string
	audience      string
	accessSecret  []byte
	refreshSecret []byte
}

func NewJWTManager(issuer, audience, accessSecret, refreshSecret string) *JWTManager {
	return &JWTManager{
		issuer:        issuer,
		audience:      audience,
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
	}
}

func (m *JWTManager) SignAccessToken(userID uint, roles, perms []string, ttl time.Duration) (string, error) {
	return m.SignAccessTokenWithJTI(userID, roles, perms, ttl, uuid.NewString())
}

func (m *JWTManager) SignAccessTokenWithJTI(userID uint, roles, perms []string, ttl time.Duration, jti string) (string, error) {
	if jti == "" {
		jti = uuid.NewString()
	}
	claims := Claims{
		TokenType:   "access",
		Roles:       roles,
		Permissions: perms,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			Audience:  []string{m.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.accessSecret)
}

func (m *JWTManager) SignRefreshToken(userID uint, ttl time.Duration) (string, error) {
	claims := Claims{
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			Audience:  []string{m.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.refreshSecret)
}

func (m *JWTManager) ParseAccessToken(raw string) (*Claims, error) {
	return m.parse(raw, m.accessSecret, "access")
}

func (m *JWTManager) ParseRefreshToken(raw string) (*Claims, error) {
	return m.parse(raw, m.refreshSecret, "refresh")
}

func (m *JWTManager) parse(raw string, secret []byte, tokenType string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing algorithm")
		}
		return secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience))
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != tokenType {
		return nil, fmt.Errorf("unexpected token type: %s", claims.TokenType)
	}
	return claims, nil
}
