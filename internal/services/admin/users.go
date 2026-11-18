package admin

import (
	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

func (h *handlers) listUsers(c *gin.Context) {
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	size, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if h.clients.Identity == nil {
		httpx.Error(c, apperr.Unavailable("identity client is not configured", nil))
		return
	}
	reply, err := h.clients.Identity.ListUsers(c.Request.Context(), &pb.ListUsersRequest{
		Page: page, PageSize: size, Keyword: c.Query("keyword"), Status: c.Query("status"),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"users": reply.GetUsers(), "total": reply.GetTotal()})
}

type setUserStatusRequest struct {
	Status string `json:"status"`
}

func (h *handlers) setUserStatus(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in setUserStatusRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if in.Status == "DISABLED" {
		if claims, ok := auth.FromContext(c.Request.Context()); ok && int64(claims.UserID) == id {
			httpx.Error(c, apperr.Invalid("cannot disable the current admin account"))
			return
		}
	}
	if h.clients.Identity == nil {
		httpx.Error(c, apperr.Unavailable("identity client is not configured", nil))
		return
	}
	user, err := h.clients.Identity.SetUserStatus(c.Request.Context(), &pb.SetUserStatusRequest{
		UserId: id, Status: in.Status,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, user)
}
