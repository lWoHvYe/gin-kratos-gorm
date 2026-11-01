package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/config"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID uint64   `json:"uid"`
	Roles  []string `json:"roles,omitempty"`
	Type   string   `json:"typ"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(c *config.Config) *Manager {
	return &Manager{secret: []byte(c.JWTSecret), issuer: c.JWTIssuer, accessTTL: c.AccessTTL, refreshTTL: c.RefreshTTL}
}

func (m *Manager) Generate(userID uint64, roles []string) (access, refresh string, err error) {
	now := time.Now()
	accessClaims := Claims{UserID: userID, Roles: roles, Type: "access", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: m.issuer, Subject: fmt.Sprintf("%d", userID), IssuedAt: jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
	}}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(m.secret)
	if err != nil {
		return "", "", apperr.Internal("sign access token", err)
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", apperr.Internal("generate refresh token", err)
	}
	return access, hex.EncodeToString(b[:]), nil
}

func (m *Manager) Parse(raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, apperr.Unauthenticated("token is empty")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperr.Unauthenticated("token expired")
		}
		return nil, apperr.Unauthenticated("invalid token")
	}
	if !token.Valid || claims.UserID == 0 || claims.Type != "access" {
		return nil, apperr.Unauthenticated("invalid token")
	}
	return claims, nil
}

func (m *Manager) ParseBearer(header string) (*Claims, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, apperr.Unauthenticated("authorization header must be Bearer <token>")
	}
	return m.Parse(parts[1])
}

func HashPassword(p string) (string, error) {
	if len(p) < 8 {
		return "", apperr.Invalid("password must be at least 8 characters")
	}
	if len(p) > 72 {
		return "", apperr.Invalid("password too long")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.Internal("hash password", err)
	}
	return string(h), nil
}
func CheckPassword(hash, p string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

type ctxKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}
func FromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(*Claims)
	return c, ok && c != nil
}
func UserID(ctx context.Context) (uint64, bool) {
	c, ok := FromContext(ctx)
	if !ok {
		return 0, false
	}
	return c.UserID, c.UserID != 0
}
func HasRole(ctx context.Context, role string) bool {
	c, ok := FromContext(ctx)
	if !ok {
		return false
	}
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func HashRefresh(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }
