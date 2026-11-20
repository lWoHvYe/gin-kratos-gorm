// Package admin implements the back-office HTTP BFF of the mall. It owns no
// persistence: every request is authenticated with the shared JWT secret,
// requires the "admin" role, and is forwarded to the owning gRPC service.
package admin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/activecache"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

const (
	// requestTimeout bounds one BFF request, including every downstream call.
	requestTimeout = 15 * time.Second
	// adminRole is required by every /api/admin/v1 route.
	adminRole = "admin"
)

// serviceAddrs are the direct gRPC addresses used by the BFF. They mirror the
// port table in docs/contracts.md and can be overridden with <SERVICE>_ADDR
// (for example CATALOG_ADDR); Kubernetes sets these to Service DNS names.
var serviceAddrs = map[string]string{
	"catalog":     "catalog:9002",
	"identity":    "identity:9001",
	"inventory":   "inventory:9003",
	"promotion":   "pricing:9005",
	"order":       "order:9006",
	"payment":     "payment:9007",
	"fulfillment": "fulfillment:9008",
	"aftersales":  "aftersales:9009",
	"seckill":     "seckill:9010",
	"ai":          "ai:9011",
}

// Clients bundles the downstream gRPC clients used by the admin BFF.
type Clients struct {
	Catalog     pb.CatalogServiceClient
	Inventory   pb.InventoryServiceClient
	Order       pb.OrderServiceClient
	Payment     pb.PaymentServiceClient
	Fulfillment pb.FulfillmentServiceClient
	AfterSales  pb.AfterSalesServiceClient
	Promotion   pb.PromotionServiceClient
	Seckill     pb.SeckillServiceClient
	AI          pb.AIServiceClient
	Identity    pb.IdentityServiceClient

	conns map[string]*grpc.ClientConn
}

// Close releases every client connection.
func (c *Clients) Close() error {
	var first error
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// States reports the gRPC connectivity state of every downstream client.
func (c *Clients) States() map[string]string {
	out := make(map[string]string, len(c.conns))
	for name, conn := range c.conns {
		if conn == nil {
			continue
		}
		out[name] = conn.GetState().String()
	}
	return out
}

// Dial opens the downstream client connections using Kubernetes Service DNS or
// <SERVICE>_ADDR overrides.
func Dial(ctx context.Context) (*Clients, error) {
	clients := &Clients{conns: map[string]*grpc.ClientConn{}}
	targets := []struct {
		name string
		bind func(*grpc.ClientConn)
	}{
		{"catalog", func(conn *grpc.ClientConn) { clients.Catalog = pb.NewCatalogServiceClient(conn) }},
		{"inventory", func(conn *grpc.ClientConn) { clients.Inventory = pb.NewInventoryServiceClient(conn) }},
		{"order", func(conn *grpc.ClientConn) { clients.Order = pb.NewOrderServiceClient(conn) }},
		{"payment", func(conn *grpc.ClientConn) { clients.Payment = pb.NewPaymentServiceClient(conn) }},
		{"fulfillment", func(conn *grpc.ClientConn) { clients.Fulfillment = pb.NewFulfillmentServiceClient(conn) }},
		{"aftersales", func(conn *grpc.ClientConn) { clients.AfterSales = pb.NewAfterSalesServiceClient(conn) }},
		{"promotion", func(conn *grpc.ClientConn) { clients.Promotion = pb.NewPromotionServiceClient(conn) }},
		{"seckill", func(conn *grpc.ClientConn) { clients.Seckill = pb.NewSeckillServiceClient(conn) }},
		{"ai", func(conn *grpc.ClientConn) { clients.AI = pb.NewAIServiceClient(conn) }},
		{"identity", func(conn *grpc.ClientConn) { clients.Identity = pb.NewIdentityServiceClient(conn) }},
	}
	for _, target := range targets {
		conn, err := grpcx.Dial(ctx, targetFor(target.name))
		if err != nil {
			_ = clients.Close()
			return nil, fmt.Errorf("dial %s: %w", target.name, err)
		}
		clients.conns[target.name] = conn
		target.bind(conn)
	}
	return clients, nil
}

// targetFor returns the gRPC target for one downstream service.
func targetFor(name string) string {
	if addr := strings.TrimSpace(os.Getenv(strings.ToUpper(name) + "_ADDR")); addr != "" {
		return addr
	}
	if addr, ok := serviceAddrs[name]; ok {
		return addr
	}
	return name + ":9000"
}

// NewEngine builds the admin Gin engine. Every route requires a valid bearer
// token with the "admin" role.
func NewEngine(logger *slog.Logger, jwt *auth.Manager, clients *Clients, aiEnabled bool) *gin.Engine {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handlers{clients: clients, aiEnabled: aiEnabled, active: activecache.New(5 * time.Second)}

	engine := gin.New()
	engine.Use(bff.Recovery(), httpx.RequestID(), httpx.AccessLog(logger), httpx.CORS(), httpx.MaxPage(1000), bff.Timeout(requestTimeout))
	engine.NoRoute(func(c *gin.Context) { httpx.Error(c, apperr.NotFound("route not found")) })
	engine.GET("/healthz", h.health)
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	engine.GET("/readyz", bff.Readiness(logger, clients.States, []string{"order", "payment"}))

	admin := engine.Group("/api/admin/v1", httpx.Auth(jwt), httpx.RequireRole(adminRole), h.requireActiveStrict, audit(logger))
	if aiEnabled {
		admin.POST("/ai/chat", h.aiChat)
		admin.POST("/ai/campaign-draft", h.startCampaignDraft)
		admin.POST("/ai/campaign-draft/:checkpoint/confirm", h.confirmCampaignDraft)
		admin.POST("/ai/campaign-draft/:checkpoint/abort", h.abortCampaignDraft)
	}
	admin.GET("/users", h.listUsers)
	admin.PUT("/users/:id/status", h.setUserStatus)
	admin.GET("/categories", h.listCategoriesAdmin)
	admin.POST("/categories", h.createCategory)
	admin.PUT("/categories/:id", h.updateCategory)
	admin.DELETE("/categories/:id", h.deleteCategory)
	admin.GET("/brands", h.listBrands)
	admin.POST("/brands", h.createBrand)
	admin.PUT("/brands/:id", h.updateBrand)
	admin.DELETE("/brands/:id", h.deleteBrand)
	admin.GET("/products", h.listAdminProducts)
	admin.GET("/products/:id", h.getAdminProduct)
	admin.POST("/products", h.createProduct)
	admin.PUT("/products/:id", h.updateProduct)
	admin.DELETE("/products/:id", h.deleteProduct)
	admin.PUT("/products/:id/status", h.setProductStatus)
	admin.POST("/products/:id/skus", h.createSku)
	admin.PUT("/skus/:id", h.updateSku)
	admin.DELETE("/skus/:id", h.deleteSku)
	admin.PUT("/skus/:id/status", h.setSkuStatus)
	admin.GET("/inventory/stocks", h.listStocks)
	admin.POST("/inventory/adjust", h.adjustStock)
	admin.GET("/orders", h.listOrders)
	admin.GET("/orders/:orderNo", h.getOrder)
	admin.POST("/orders/:orderNo/cancel", h.cancelOrder)
	admin.POST("/payments/:paymentNo/verify", h.verifyPayment)
	admin.GET("/shipments", h.listShipments)
	admin.GET("/shipments/:shipmentNo", h.getShipment)
	admin.POST("/shipments", h.createShipment)
	admin.POST("/shipments/:shipmentNo/ship", h.shipShipment)
	admin.GET("/after-sales", h.listAfterSales)
	admin.POST("/after-sales/:no/approve", h.approveAfterSale)
	admin.POST("/after-sales/:no/receive", h.receiveReturn)
	admin.POST("/after-sales/:no/refund", h.refundAfterSale)
	admin.GET("/payments/refunds", h.listRefunds)
	admin.GET("/ai/feedback/summary", h.aiFeedbackSummary)
	admin.GET("/ai/rag/search", h.ragSearch)
	admin.GET("/ai/ops/brief", h.opsBrief)
	admin.GET("/ai/ops/latest", h.latestOpsBrief)
	admin.POST("/ai/ops/feedback", h.opsFeedback)
	admin.GET("/campaigns", h.listCampaigns)
	admin.POST("/campaigns", h.createCampaign)
	admin.PUT("/campaigns/:id", h.updateCampaign)
	admin.POST("/campaigns/:id/status", h.setCampaignStatus)
	admin.GET("/promotions", h.listPromotions)
	admin.POST("/promotions", h.createPromotion)
	admin.PUT("/promotions/:id", h.updatePromotion)
	admin.POST("/promotions/:id/status", h.setPromotionStatus)
	admin.GET("/promotion-reservations", h.listPromotionReservations)
	admin.GET("/seckill/activities", h.listSeckillActivities)
	admin.POST("/seckill/activities", h.createSeckillActivity)
	admin.GET("/seckill/activities/:id", h.getSeckillActivity)
	admin.PUT("/seckill/activities/:id", h.updateSeckillActivity)
	admin.POST("/seckill/activities/:id/publish", h.publishSeckillActivity)
	admin.POST("/seckill/activities/:id/pause", h.pauseSeckillActivity)
	admin.POST("/seckill/activities/:id/warmup", h.warmupSeckillActivity)
	admin.GET("/seckill/activities/:id/stock", h.getSeckillStock)
	admin.POST("/seckill/dlq/replay", h.replaySeckillDLQ)
	admin.POST("/promotions/evaluate", h.evaluatePromotions)
	admin.POST("/promotions/evaluate-draft", h.evaluateDraft)
	admin.POST("/promotions/apply-draft", h.applyDraft)

	return engine
}

// audit records privileged mutations with the acting admin and request id.
// Read-only calls are skipped to keep the log signal high.
func audit(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		c.Next()
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return
		}
		uid, _ := auth.UserID(c.Request.Context())
		logger.Info("admin audit", "user_id", uid, "method", method,
			"path", c.FullPath(), "status", c.Writer.Status(),
			"request_id", httpx.RequestIDFrom(c))
	}
}

// handlers holds the downstream clients. It is stateless.
type handlers struct {
	clients   *Clients
	aiEnabled bool
	active    *activecache.Cache
}

// ---------------------------------------------------------------- middleware

// recovery converts a panic into the regular JSON error envelope.
// timeout caps the request context so a slow downstream cannot pin a request
// forever.
// ---------------------------------------------------------------- routes

func (h *handlers) health(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) }

// ---------------------------------------------------------------- helpers

// bindJSON decodes the request body into dst and maps decoding failures onto an
// invalid-argument error.
// pathID parses a positive int64 path parameter.
// queryInt64 parses an optional int64 query parameter; an absent value is 0.
// queryInt32 parses an optional int32 query parameter; an absent value is 0.
// remoteError translates a downstream gRPC status into the domain error model
// so httpx.Error emits the right HTTP status. Domain errors pass through.
