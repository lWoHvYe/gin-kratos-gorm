package gateway

import (
	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

func (h *handlers) getAIPreferences(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.AI.GetPreferences(c.Request.Context(), &pb.GetAIPreferencesRequest{UserId: uid})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) setAIPreferences(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var prefs pb.AIPreferences
	if err := bff.BindJSON(c, &prefs); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.AI.SetPreferences(c.Request.Context(), &pb.SetAIPreferencesRequest{
		UserId: uid, Preferences: &prefs,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) clearAIPreferences(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.AI.ClearPreferences(c.Request.Context(), &pb.ClearAIPreferencesRequest{UserId: uid}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"cleared": true})
}
