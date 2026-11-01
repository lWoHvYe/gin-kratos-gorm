package auth

import (
	"testing"
	"time"

	"gin-kratos-gorm/internal/conf"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/types/known/durationpb"
)

func testManager(secret string, ttl time.Duration) *Manager {
	return NewManager(&conf.Auth{
		JwtSecret:      secret,
		JwtIssuer:      "test-issuer",
		AccessTokenTtl: durationpb.New(ttl),
	})
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "correct-horse") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password to fail")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	m := testManager("0123456789abcdef", time.Hour)
	token, expireAt, err := m.Generate(7, "alice", []uint64{2}, []string{"user"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if token == "" || !expireAt.After(time.Now()) {
		t.Fatal("invalid token/expiry")
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || len(claims.RoleCodes) != 1 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := m.ParseBearer("Bearer " + token); err != nil {
		t.Fatalf("parse bearer: %v", err)
	}
	if _, err := m.ParseBearer("Basic " + token); err == nil {
		t.Fatal("expected invalid scheme")
	}
}

func TestTokenRejectsWrongSecretAndIssuer(t *testing.T) {
	issuer := testManager("0123456789abcdef", time.Hour)
	token, _, err := issuer.Generate(7, "alice", nil, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	other := testManager("fedcba9876543210", time.Hour)
	if _, err := other.Parse(token); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
	otherIssuer := NewManager(&conf.Auth{
		JwtSecret:      "0123456789abcdef",
		JwtIssuer:      "another-issuer",
		AccessTokenTtl: durationpb.New(time.Hour),
	})
	if _, err := otherIssuer.Parse(token); err == nil {
		t.Fatal("expected wrong issuer to fail")
	}
}

func TestExpiredToken(t *testing.T) {
	m := testManager("0123456789abcdef", time.Hour)
	claims := Claims{
		UserID: 7,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := m.Parse(token); err == nil {
		t.Fatal("expected expired token to fail")
	}
}
