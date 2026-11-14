package gateway

import (
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

func (h *handlers) register(c *gin.Context) {
	var req pb.RegisterRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Register(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, reply)
}

func (h *handlers) login(c *gin.Context) {
	var req pb.LoginRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Login(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *handlers) refresh(c *gin.Context) {
	var in refreshRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Refresh(c.Request.Context(), &pb.RefreshRequest{RefreshToken: in.RefreshToken})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) logout(c *gin.Context) {
	var in refreshRequest
	if bff.BodyPresent(c) {
		if err := bff.BindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	if _, err := h.clients.Identity.Logout(c.Request.Context(), &pb.LogoutRequest{RefreshToken: in.RefreshToken}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "logged out"})
}
