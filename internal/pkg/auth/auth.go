// Package auth centralizes password hashing, JWT issuing/parsing and the
// authenticated identity stored in request contexts. Both Gin and gRPC extract
// the same Claims type, so downstream use cases are transport agnostic.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/apperr"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultIssuer is used when auth.jwt_issuer is empty.
	DefaultIssuer = "gin-kratos-gorm"
	// MinPasswordLength is the minimum accepted password length.
	MinPasswordLength = 8
)

// Claims is the JWT payload. RoleIDs map to the roles table; RoleCodes are the
// stable codes ("admin", "user") used by RBAC checks.
type Claims struct {
	UserID    uint64   `json:"uid"`
	Username  string   `json:"username"`
	RoleIDs   []uint64 `json:"roles,omitempty"`
	RoleCodes []string `json:"role_codes,omitempty"`
	jwt.RegisteredClaims
}

// Manager issues and validates access tokens.
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewManager(cfg *conf.Auth) *Manager {
	issuer := DefaultIssuer
	ttl := 24 * time.Hour
	if cfg != nil {
		if cfg.JwtIssuer != "" {
			issuer = cfg.JwtIssuer
		}
		if cfg.AccessTokenTtl != nil && cfg.AccessTokenTtl.AsDuration() > 0 {
			ttl = cfg.AccessTokenTtl.AsDuration()
		}
	}
	var secret []byte
	if cfg != nil {
		secret = []byte(cfg.JwtSecret)
	}
	return &Manager{secret: secret, issuer: issuer, ttl: ttl}
}

func (m *Manager) TTL() time.Duration { return m.ttl }

// Generate creates a signed access token and returns its expiry.
func (m *Manager) Generate(userID uint64, username string, roleIDs []uint64, roleCodes []string) (string, time.Time, error) {
	now := time.Now()
	expireAt := now.Add(m.ttl)
	claims := Claims{
		UserID:    userID,
		Username:  username,
		RoleIDs:   roleIDs,
		RoleCodes: roleCodes,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expireAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, apperr.Internal("sign token", err)
	}
	return signed, expireAt, nil
}

// Parse validates a raw bearer token.
func (m *Manager) Parse(raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, apperr.Unauthenticated("token is empty")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, apperr.Unauthenticated("token expired")
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, apperr.Unauthenticated("token not active yet")
		default:
			return nil, apperr.Unauthenticated("invalid token")
		}
	}
	if token == nil || !token.Valid || claims.UserID == 0 {
		return nil, apperr.Unauthenticated("invalid token")
	}
	return claims, nil
}

// ParseBearer extracts a token from an "Authorization: Bearer xxx" header.
func (m *Manager) ParseBearer(header string) (*Claims, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, apperr.Unauthenticated("authorization header must be 'Bearer <token>'")
	}
	return m.Parse(parts[1])
}

type claimsKey struct{}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*Claims)
	return claims, ok && claims != nil
}

// UserID returns the authenticated user id.
func UserID(ctx context.Context) (uint64, bool) {
	claims, ok := FromContext(ctx)
	if !ok || claims.UserID == 0 {
		return 0, false
	}
	return claims.UserID, true
}

// MustUserID returns 0 when the request is unauthenticated. Callers should
// enforce authentication at the transport boundary.
func MustUserID(ctx context.Context) uint64 {
	id, _ := UserID(ctx)
	return id
}

// HasRole reports whether the authenticated user has one of the given role codes.
func HasRole(ctx context.Context, codes ...string) bool {
	claims, ok := FromContext(ctx)
	if !ok {
		return false
	}
	for _, have := range claims.RoleCodes {
		for _, want := range codes {
			if have == want {
				return true
			}
		}
	}
	return false
}

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", apperr.Invalid("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > 72 {
		return "", apperr.Invalid("password must be at most 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.Internal("hash password", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
