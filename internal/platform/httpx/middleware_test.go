package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://admin.example.com, https://shop.example.com")
	engine := gin.New()
	engine.Use(CORS())
	engine.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		origin string
		want   string
	}{
		{"https://admin.example.com", "https://admin.example.com"},
		{"https://shop.example.com", "https://shop.example.com"},
		{"https://evil.example.com", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Origin", tc.origin)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Fatalf("origin %q -> %q, want %q", tc.origin, got, tc.want)
		}
	}
}

func TestCORSRejectsWhenUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	engine := gin.New()
	engine.Use(CORS())
	engine.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unconfigured CORS must not reflect, got %q", got)
	}
}

func TestMaxPageRejectsDeepPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(MaxPage(1000))
	engine.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, tc := range []struct {
		page string
		want int
	}{
		{"1", http.StatusOK},
		{"1000", http.StatusOK},
		{"1001", http.StatusBadRequest},
		{"-1", http.StatusBadRequest},
		{"abc", http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodGet, "/x?page="+tc.page, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("page %q -> %d, want %d", tc.page, rec.Code, tc.want)
		}
	}
}
