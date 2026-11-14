// Package gateway implements the public HTTP BFF of the mall. It owns no
// persistence: requests are authenticated locally with the shared JWT secret,
// mapped onto the generated protobuf request types, and forwarded to the
// service that owns the data.

package gateway

import (
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options toggles BFF capabilities that must not be on in production.
type Options struct {
	// PaymentMockEnabled exposes POST /payments/:paymentNo/pay, which settles
	// an intent with the mock channel. Production channels call the payment
	// service directly with a signature.
	PaymentMockEnabled bool
}

// NewEngine builds the public gateway Gin engine. The routes under /api/v1 that
// are not explicitly public require a valid bearer token.
func NewEngine(logger *slog.Logger, jwt *auth.Manager, clients *Clients, opts Options) *gin.Engine {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handlers{clients: clients, log: logger}

	engine := gin.New()
	engine.Use(bff.Recovery(), httpx.RequestID(), httpx.AccessLog(logger), httpx.CORS(), bff.Timeout(requestTimeout))
	engine.NoRoute(func(c *gin.Context) { httpx.Error(c, apperr.NotFound("route not found")) })
	engine.GET("/healthz", h.health)
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	engine.GET("/readyz", bff.Readiness(logger, clients.States, []string{"identity", "order", "payment"}))

	engine.POST("/api/v1/auth/register", h.register)
	engine.POST("/api/v1/auth/login", h.login)
	engine.POST("/api/v1/auth/refresh", h.refresh)
	engine.POST("/api/v1/auth/logout", h.logout)
	engine.GET("/api/v1/categories", h.listCategories)
	engine.GET("/api/v1/brands", h.listBrands)
	engine.GET("/api/v1/products", h.listProducts)
	engine.GET("/api/v1/products/:id", h.getProduct)
	engine.GET("/api/v1/skus/:id", h.getSku)
	engine.GET("/api/v1/seckill/activities", h.listPublicSeckillActivities)
	engine.GET("/api/v1/seckill/activities/:id", h.getSeckillActivity)

	authed := engine.Group("/api/v1", httpx.Auth(jwt), requireUser())
	authed.GET("/addresses", h.listAddresses)
	authed.POST("/addresses", h.createAddress)
	authed.PUT("/addresses/:id", h.updateAddress)
	authed.DELETE("/addresses/:id", h.deleteAddress)
	authed.POST("/addresses/:id/default", h.setDefaultAddress)
	authed.GET("/cart", h.getCart)
	authed.POST("/cart/items", h.addCartItem)
	authed.PATCH("/cart/items", h.updateCartItem)
	authed.DELETE("/cart/items", h.removeCartItem)
	authed.POST("/cart/merge", h.mergeCart)
	authed.POST("/checkout/preview", h.previewCheckout)
	authed.POST("/checkout/place", h.placeOrder)
	authed.GET("/orders", h.listOrders)
	authed.GET("/orders/:orderNo", h.getOrder)
	authed.POST("/orders/:orderNo/cancel", h.cancelOrder)
	authed.POST("/orders/:orderNo/receive", h.receiveOrder)
	if opts.PaymentMockEnabled {
		authed.POST("/payments/:paymentNo/pay", h.pay)
	}
	authed.POST("/after-sales", h.createAfterSale)
	authed.GET("/after-sales", h.listAfterSales)
	authed.GET("/after-sales/:no", h.getAfterSale)
	authed.POST("/after-sales/:no/return-tracking", h.submitReturnTracking)
	authed.GET("/orders/:orderNo/shipments", h.getOrderShipment)
	authed.POST("/seckill/activities/:id/reserve", h.reserveSeckill)
	authed.GET("/seckill/qualifications/:token", h.getSeckillQualification)

	return engine
}

// handlers holds the downstream clients. It is stateless.

func (h *handlers) health(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) }
