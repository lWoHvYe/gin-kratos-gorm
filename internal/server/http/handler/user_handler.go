package handler

import (
	"net/http"
	"strconv"

	pb "gin-kratos-gorm/api/user/v1"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/httpx"
	"gin-kratos-gorm/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req pb.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", err))
		return
	}
	reply, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.Created(c, reply)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req pb.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", err))
		return
	}
	reply, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID, ok := auth.UserID(c.Request.Context())
	if !ok {
		httpx.Error(c, apperr.Unauthenticated("login required"))
		return
	}
	reply, err := h.svc.GetByID(c.Request.Context(), &pb.SingleIDRequest{Id: userID})
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, reply)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid user id"))
		return
	}
	reply, svcErr := h.svc.GetByID(c.Request.Context(), &pb.SingleIDRequest{Id: id})
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	httpx.OK(c, reply)
}

func (h *UserHandler) UpdateRoleByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httpx.Error(c, apperr.Invalid("invalid user id"))
		return
	}
	var req pb.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Invalid("invalid request body: %v", err))
		return
	}
	req.Id = id
	reply, svcErr := h.svc.UpdateRoleByID(c.Request.Context(), &req)
	if svcErr != nil {
		httpx.Error(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, reply)
}
