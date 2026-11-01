package server

import (
	"context"
	"net/http"
	"time"

	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/data/persistence"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/httpx"
	"gin-kratos-gorm/internal/pkg/metrics"
	httpServerHandler "gin-kratos-gorm/internal/server/http/handler"
	"gin-kratos-gorm/internal/server/http/router"
	"gin-kratos-gorm/internal/server/middleware"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer builds the all-in-one REST API.
func NewHTTPServer(
	cfg *conf.Bootstrap,
	userHandler *httpServerHandler.UserHandler,
	productHandler *httpServerHandler.ProductHandler,
	orderHandler *httpServerHandler.OrderHandler,
	data *persistence.Data,
	enforcer *casbin.Enforcer,
	jwtManager *auth.Manager,
	logger log.Logger,
) *khttp.Server {
	r := newGin(cfg, data, logger)
	public := r.Group("/api")
	private := r.Group("/api")
	private.Use(middleware.JWTAuth(jwtManager), middleware.Casbin(enforcer))
	admin := private.Group("")
	admin.Use(middleware.RequireRole("admin"))

	router.NewUserRouter(userHandler).InitUserRoutes(public, private, admin)
	router.NewProductRouter(productHandler).RegisterProductRouter(public, admin)
	router.NewOrderRouter(orderHandler).RegisterRouter(private)
	return wrapHTTPServer(cfg, r)
}

// NewOrderHTTPServer is the order-only deployment.
func NewOrderHTTPServer(
	cfg *conf.Bootstrap,
	orderHandler *httpServerHandler.OrderHandler,
	data *persistence.Data,
	jwtManager *auth.Manager,
	logger log.Logger,
) *khttp.Server {
	r := newGin(cfg, data, logger)
	private := r.Group("/api")
	private.Use(middleware.JWTAuth(jwtManager))
	router.NewOrderRouter(orderHandler).RegisterRouter(private)
	return wrapHTTPServer(cfg, r)
}

// NewProductHTTPServer is the product-only deployment.
func NewProductHTTPServer(
	cfg *conf.Bootstrap,
	productHandler *httpServerHandler.ProductHandler,
	data *persistence.Data,
	enforcer *casbin.Enforcer,
	jwtManager *auth.Manager,
	logger log.Logger,
) *khttp.Server {
	r := newGin(cfg, data, logger)
	public := r.Group("/api")
	private := r.Group("/api")
	private.Use(middleware.JWTAuth(jwtManager), middleware.Casbin(enforcer))
	admin := private.Group("")
	admin.Use(middleware.RequireRole("admin"))
	router.NewProductRouter(productHandler).RegisterProductRouter(public, admin)
	return wrapHTTPServer(cfg, r)
}

func newGin(cfg *conf.Bootstrap, data *persistence.Data, logger log.Logger) *gin.Engine {
	if cfg.App != nil && cfg.App.Env != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLog(logger))
	r.Use(cors())

	r.GET("/healthz", func(c *gin.Context) {
		httpx.OK(c, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := data.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}
		httpx.OK(c, gin.H{"status": "ready"})
	})
	if cfg.App == nil || cfg.App.MetricsEnabled {
		r.GET("/metrics", gin.WrapH(metrics.Default().Handler()))
	}
	return r
}

func wrapHTTPServer(cfg *conf.Bootstrap, r *gin.Engine) *khttp.Server {
	opts := make([]khttp.ServerOption, 0, 4)
	if cfg.Server.Http.Network != "" {
		opts = append(opts, khttp.Network(cfg.Server.Http.Network))
	}
	if cfg.Server.Http.Addr != "" {
		opts = append(opts, khttp.Address(cfg.Server.Http.Addr))
	}
	if cfg.Server.Http.Timeout != nil {
		opts = append(opts, khttp.Timeout(cfg.Server.Http.Timeout.AsDuration()))
	}
	srv := khttp.NewServer(opts...)
	srv.HandlePrefix("/", r)
	return srv
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
