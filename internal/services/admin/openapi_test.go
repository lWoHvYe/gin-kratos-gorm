package admin

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// TestOpenAPIAdminRoutesExist keeps docs/openapi/admin.yaml honest in the
// reverse direction: every documented route must still be served. Undocumented
// routes are allowed (tracked in docs/architecture-review.md) so the spec can
// grow incrementally without blocking on documentation.
func TestOpenAPIAdminRoutesExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := auth.NewManager(&config.Config{
		JWTSecret: "0123456789abcdef0123456789abcdef", JWTIssuer: "mall",
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	engine := NewEngine(nil, manager, &Clients{})

	registered := make(map[string]bool, len(engine.Routes()))
	for _, route := range engine.Routes() {
		registered[route.Method+" "+normalizeParams(route.Path)] = true
	}

	spec := readOpenAPIKeys(t, "../../../docs/openapi/admin.yaml")
	if len(spec) == 0 {
		t.Fatal("admin.yaml has no paths")
	}
	for key := range spec {
		if !registered[key] {
			t.Errorf("documented route is not served: %s", key)
		}
	}
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

func normalizeParams(path string) string {
	path = pathParam.ReplaceAllString(path, ":param")
	// Gin also uses :name; collapse it so spec and router compare equal.
	return regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*`).ReplaceAllString(path, ":param")
}

func readOpenAPIKeys(t *testing.T, file string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	out := make(map[string]bool)
	verbs := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	for path, ops := range doc.Paths {
		for method := range ops {
			if verbs[strings.ToLower(method)] {
				out[strings.ToUpper(method)+" "+normalizeParams(path)] = true
			}
		}
	}
	return out
}
