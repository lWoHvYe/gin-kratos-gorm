package router

import (
	"gin-kratos-gorm/internal/server/http/handler"

	"github.com/gin-gonic/gin"
)

type UserRouter struct {
	userHandler *handler.UserHandler
}

func NewUserRouter(h *handler.UserHandler) *UserRouter {
	return &UserRouter{userHandler: h}
}

func (r *UserRouter) InitUserRoutes(public, private, admin *gin.RouterGroup) {
	base := public.Group("/base")
	{
		base.POST("/register", r.userHandler.Register)
		base.POST("/login", r.userHandler.Login)
	}

	private.GET("/users/me", r.userHandler.GetMe)

	admin.POST("/users", r.userHandler.Register)
	admin.GET("/users/:id", r.userHandler.GetByID)
	admin.PUT("/users/:id/role", r.userHandler.UpdateRoleByID)
}
