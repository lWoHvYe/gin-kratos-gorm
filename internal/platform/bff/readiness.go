package bff

import (
	"log/slog"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
)

// Readiness reports dependency connectivity. Only critical dependencies fail the
// probe; everything else is reported as degraded so one slow service does not
// pull every BFF replica out of rotation.
func Readiness(logger *slog.Logger, states func() map[string]string, critical []string) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	isCritical := make(map[string]bool, len(critical))
	for _, name := range critical {
		isCritical[name] = true
	}
	return func(c *gin.Context) {
		current := states()
		names := make([]string, 0, len(current))
		for name := range current {
			names = append(names, name)
		}
		sort.Strings(names)

		status := "ready"
		code := http.StatusOK
		failed := make([]string, 0)
		degraded := make([]string, 0)
		for _, name := range names {
			if healthyState(current[name]) {
				continue
			}
			if isCritical[name] {
				failed = append(failed, name)
			} else {
				degraded = append(degraded, name)
			}
		}
		if len(failed) > 0 {
			status = "not ready"
			code = http.StatusServiceUnavailable
			logger.Warn("readiness failed", "dependencies", failed)
		} else if len(degraded) > 0 {
			status = "degraded"
			logger.Warn("readiness degraded", "dependencies", degraded)
		}
		c.JSON(code, gin.H{
			"status":       status,
			"dependencies": current,
			"failed":       failed,
			"degraded":     degraded,
		})
	}
}

// healthyState treats an idle/connecting channel as usable: gRPC dials lazily
// and a not-yet-used connection is not a failure.
func healthyState(state string) bool {
	switch state {
	case "IDLE", "CONNECTING", "READY":
		return true
	default:
		return false
	}
}
