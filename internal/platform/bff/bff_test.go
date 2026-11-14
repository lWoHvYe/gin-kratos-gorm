package bff

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRemoteErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want apperr.Code
	}{
		{status.Error(codes.InvalidArgument, "bad"), apperr.CodeInvalidArgument},
		{status.Error(codes.NotFound, "gone"), apperr.CodeNotFound},
		{status.Error(codes.PermissionDenied, "no"), apperr.CodePermissionDenied},
		{status.Error(codes.ResourceExhausted, "slow down"), apperr.CodeRateLimited},
		{status.Error(codes.Unavailable, "down"), apperr.CodeUnavailable},
		{status.Error(codes.Internal, "boom"), apperr.CodeInternal},
	}
	for _, tc := range cases {
		if got := RemoteError(tc.err); !apperr.IsCode(got, tc.want) {
			t.Fatalf("RemoteError(%v) code = %v, want %s", tc.err, got, tc.want)
		}
	}
	if RemoteError(nil) != nil {
		t.Fatal("nil error must stay nil")
	}
}

func TestBindJSONRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/x", func(c *gin.Context) {
		var body map[string]any
		if err := BindJSON(c, &body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})
	payload := `{"a":"` + strings.Repeat("x", MaxBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("oversized body was accepted")
	}
}

func TestPathIDRejectsNonPositive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: "0"}}
	if _, err := PathID(c, "id"); err == nil {
		t.Fatal("zero id must be rejected")
	}
	c.Params = gin.Params{{Key: "id", Value: "42"}}
	if id, err := PathID(c, "id"); err != nil || id != 42 {
		t.Fatalf("id = %d, %v", id, err)
	}
}
