package gateway

import (
	"net/http/httptest"
	"testing"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTargetFor(t *testing.T) {
	t.Setenv("IDENTITY_ADDR", "127.0.0.1:19001")
	if got := targetFor("identity"); got != "127.0.0.1:19001" {
		t.Fatalf("targetFor = %q", got)
	}
	if got := targetFor("missing"); got != "missing:9000" {
		t.Fatalf("targetFor fallback = %q", got)
	}
}

func TestRemoteErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code apperr.Code
	}{
		{status.Error(codes.NotFound, "gone"), apperr.CodeNotFound},
		{status.Error(codes.InvalidArgument, "bad"), apperr.CodeInvalidArgument},
		{status.Error(codes.PermissionDenied, "no"), apperr.CodePermissionDenied},
		{status.Error(codes.Unavailable, "down"), apperr.CodeUnavailable},
	}
	for _, tc := range cases {
		err := remoteError(tc.err)
		if !apperr.IsCode(err, tc.code) {
			t.Fatalf("remoteError(%v) = %v, want %s", tc.err, err, tc.code)
		}
	}
}

func TestUserIDFromClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request = c.Request.WithContext(auth.WithClaims(c.Request.Context(), &auth.Claims{UserID: 7}))
	uid, err := userID(c)
	if err != nil || uid != 7 {
		t.Fatalf("userID = %d, %v", uid, err)
	}
}

func TestQueryParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?page=2&page_size=bad", nil)
	if page, err := queryInt32(c, "page"); err != nil || page != 2 {
		t.Fatalf("page = %d, %v", page, err)
	}
	if _, err := queryInt32(c, "page_size"); err == nil {
		t.Fatal("expected invalid page_size")
	}
}
