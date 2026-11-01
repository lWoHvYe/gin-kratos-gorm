package auth

import (
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/config"
)

func manager(secret string, ttl time.Duration) *Manager {
	return NewManager(&config.Config{JWTSecret: secret, JWTIssuer: "test", AccessTTL: ttl})
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct-horse") {
		t.Fatal("expected match")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("expected mismatch")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}

func TestJWT(t *testing.T) {
	m := manager("0123456789abcdef", time.Hour)
	access, refresh, err := m.Generate(7, []string{"user"})
	if err != nil {
		t.Fatal(err)
	}
	if access == "" || len(refresh) != 64 {
		t.Fatalf("bad tokens: %q %q", access, refresh)
	}
	c, err := m.ParseBearer("Bearer " + access)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 7 || !HasRole(WithClaims(t.Context(), c), "user") {
		t.Fatalf("bad claims: %+v", c)
	}
	if _, err := m.Parse(access); err != nil {
		t.Fatal(err)
	}
	if _, err := manager("fedcba9876543210", time.Hour).Parse(access); err == nil {
		t.Fatal("expected wrong secret failure")
	}
}
