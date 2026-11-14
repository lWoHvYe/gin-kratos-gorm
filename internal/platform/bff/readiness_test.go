package bff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func readinessEngine(states map[string]string, critical ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/readyz", Readiness(nil, func() map[string]string { return states }, critical))
	return engine
}

func TestReadinessFailsOnlyOnCriticalDependency(t *testing.T) {
	engine := readinessEngine(map[string]string{
		"identity": "READY",
		"order":    "TRANSIENT_FAILURE",
	}, "identity", "order")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("critical failure status = %d", rec.Code)
	}

	engine = readinessEngine(map[string]string{
		"identity": "READY",
		"catalog":  "TRANSIENT_FAILURE",
	}, "identity")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("degraded status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("status = %v", body["status"])
	}
}

func TestReadinessTreatsIdleAsUsable(t *testing.T) {
	engine := readinessEngine(map[string]string{"identity": "IDLE", "order": "CONNECTING"}, "identity", "order")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("idle/connecting status = %d", rec.Code)
	}
}
