// Package admin implements the back-office HTTP BFF of the mall. It owns no
// persistence: every request is authenticated with the shared JWT secret,
// requires the "admin" role, and is forwarded to the owning gRPC service.
package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	"inventory":   "inventory:9003",
	"order":       "order:9006",
	"payment":     "payment:9007",
	"fulfillment": "fulfillment:9008",
	"aftersales":  "aftersales:9009",
}

// Clients bundles the downstream gRPC clients used by the admin BFF.
type Clients struct {
	Catalog     pb.CatalogServiceClient
	Inventory   pb.InventoryServiceClient
	Order       pb.OrderServiceClient
	Payment     pb.PaymentServiceClient
	Fulfillment pb.FulfillmentServiceClient
	AfterSales  pb.AfterSalesServiceClient

	conns []*grpc.ClientConn
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

// Dial opens the downstream client connections using Kubernetes Service DNS or
// <SERVICE>_ADDR overrides.
func Dial(ctx context.Context) (*Clients, error) {
	clients := &Clients{}
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
	}
	for _, target := range targets {
		conn, err := grpcx.Dial(ctx, targetFor(target.name))
		if err != nil {
			_ = clients.Close()
			return nil, fmt.Errorf("dial %s: %w", target.name, err)
		}
		clients.conns = append(clients.conns, conn)
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
func NewEngine(logger *slog.Logger, jwt *auth.Manager, clients *Clients) *gin.Engine {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handlers{clients: clients}

	engine := gin.New()
	engine.Use(recovery(), httpx.RequestID(), httpx.AccessLog(logger), httpx.CORS(), timeout(requestTimeout))
	engine.NoRoute(func(c *gin.Context) { httpx.Error(c, apperr.NotFound("route not found")) })
	engine.GET("/healthz", h.health)

	admin := engine.Group("/api/admin/v1", httpx.Auth(jwt), httpx.RequireRole(adminRole))
	admin.POST("/categories", h.createCategory)
	admin.POST("/products", h.createProduct)
	admin.PUT("/products/:id/status", h.setProductStatus)
	admin.POST("/inventory/adjust", h.adjustStock)
	admin.GET("/orders", h.listOrders)
	admin.POST("/orders/:orderNo/cancel", h.cancelOrder)
	admin.POST("/payments/:paymentNo/verify", h.verifyPayment)
	admin.POST("/shipments", h.createShipment)
	admin.POST("/shipments/:shipmentNo/ship", h.shipShipment)
	admin.POST("/after-sales/:no/approve", h.approveAfterSale)
	admin.POST("/after-sales/:no/refund", h.refundAfterSale)

	return engine
}

// handlers holds the downstream clients. It is stateless.
type handlers struct {
	clients *Clients
}

// ---------------------------------------------------------------- middleware

// recovery converts a panic into the regular JSON error envelope.
func recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		httpx.Error(c, apperr.Internal("panic recovered", fmt.Errorf("%v", recovered)))
	})
}

// timeout caps the request context so a slow downstream cannot pin a request
// forever.
func timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ---------------------------------------------------------------- routes

func (h *handlers) health(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) }

func (h *handlers) createCategory(c *gin.Context) {
	var req pb.CreateCategoryRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	if req.Category == nil {
		httpx.Error(c, apperr.Invalid("category is required"))
		return
	}
	category, err := h.clients.Catalog.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.Created(c, category)
}

func (h *handlers) createProduct(c *gin.Context) {
	var req pb.CreateProductRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	product, err := h.clients.Catalog.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.Created(c, product)
}

// setStatusRequest is the body of PUT /products/:id/status.
type setStatusRequest struct {
	Status string `json:"status"`
}

func (h *handlers) setProductStatus(c *gin.Context) {
	productID, err := pathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in setStatusRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		httpx.Error(c, apperr.Invalid("status is required"))
		return
	}
	if _, err := h.clients.Catalog.SetProductStatus(c.Request.Context(), &pb.SetProductStatusRequest{
		Id:     productID,
		Status: status,
	}); err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, gin.H{"id": productID, "status": status})
}

// adjustStockRequest is the body of POST /inventory/adjust.
type adjustStockRequest struct {
	SkuID       int64  `json:"sku_id"`
	WarehouseID int64  `json:"warehouse_id"`
	Delta       int64  `json:"delta"`
	Reason      string `json:"reason"`
}

func (h *handlers) adjustStock(c *gin.Context) {
	var in adjustStockRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	stock, err := h.clients.Inventory.AdminAdjustStock(c.Request.Context(), &pb.AdminAdjustStockRequest{
		SkuId:       in.SkuID,
		WarehouseId: in.WarehouseID,
		Delta:       in.Delta,
		Reason:      in.Reason,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, stock)
}

func (h *handlers) listOrders(c *gin.Context) {
	page, err := queryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := queryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	userID, err := queryInt64(c, "user_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orders, err := h.clients.Order.ListOrders(c.Request.Context(), &pb.ListOrdersRequest{
		UserId:   userID,
		Admin:    true,
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, orders)
}

// createShipmentRequest is the body of POST /shipments. When "items" is omitted
// the parcel is built from every line of the order. "ship" additionally moves
// the new shipment from CREATED to SHIPPED (fulfillment.Ship).
type createShipmentRequest struct {
	OrderNo     string         `json:"order_no"`
	WarehouseID int64          `json:"warehouse_id"`
	Carrier     string         `json:"carrier"`
	TrackingNo  string         `json:"tracking_no"`
	Items       []shipmentItem `json:"items"`
	Ship        bool           `json:"ship"`
}

type shipmentItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) createShipment(c *gin.Context) {
	var in createShipmentRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(in.OrderNo)
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("order_no is required"))
		return
	}
	items := make([]*pb.ShipmentItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.ShipmentItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	if len(items) == 0 {
		// Default to a full shipment: copy the order lines through.
		order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
		if err != nil {
			httpx.Error(c, remoteError(err))
			return
		}
		for _, line := range order.GetItems() {
			items = append(items, &pb.ShipmentItem{SkuId: line.GetSkuId(), Quantity: line.GetQuantity()})
		}
	}
	shipment, err := h.clients.Fulfillment.CreateShipment(c.Request.Context(), &pb.CreateShipmentRequest{
		OrderNo:     orderNo,
		WarehouseId: in.WarehouseID,
		Carrier:     strings.TrimSpace(in.Carrier),
		TrackingNo:  strings.TrimSpace(in.TrackingNo),
		Items:       items,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	if in.Ship {
		shipment, err = h.clients.Fulfillment.Ship(c.Request.Context(), &pb.ShipRequest{ShipmentNo: shipment.GetShipmentNo()})
		if err != nil {
			httpx.Error(c, remoteError(err))
			return
		}
	}
	httpx.Created(c, shipment)
}

// approveRequest is the body of POST /after-sales/:no/approve. approved defaults
// to true so a bare approve call accepts the request.
type approveRequest struct {
	Approved   *bool  `json:"approved"`
	AmountCent int64  `json:"amount_cent"`
	Reason     string `json:"reason"`
}

type cancelOrderRequest struct {
	Reason string `json:"reason"`
}

func (h *handlers) cancelOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	var in cancelOrderRequest
	if bodyPresent(c) {
		if err := bindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	order, err := h.clients.Order.CancelOrder(c.Request.Context(), &pb.CancelOrderRequest{
		OrderNo: orderNo,
		Admin:   true,
		Reason:  strings.TrimSpace(in.Reason),
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, order)
}

func (h *handlers) verifyPayment(c *gin.Context) {
	paymentNo := strings.TrimSpace(c.Param("paymentNo"))
	if paymentNo == "" {
		httpx.Error(c, apperr.Invalid("paymentNo is required"))
		return
	}
	reply, err := h.clients.Payment.VerifyPayment(c.Request.Context(), &pb.VerifyPaymentRequest{PaymentNo: paymentNo})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) shipShipment(c *gin.Context) {
	shipmentNo := strings.TrimSpace(c.Param("shipmentNo"))
	if shipmentNo == "" {
		httpx.Error(c, apperr.Invalid("shipmentNo is required"))
		return
	}
	shipment, err := h.clients.Fulfillment.Ship(c.Request.Context(), &pb.ShipRequest{ShipmentNo: shipmentNo})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, shipment)
}

func (h *handlers) refundAfterSale(c *gin.Context) {
	no := strings.TrimSpace(c.Param("no"))
	if no == "" {
		httpx.Error(c, apperr.Invalid("after-sale number is required"))
		return
	}
	afterSale, err := h.clients.AfterSales.Refund(c.Request.Context(), &pb.RefundAfterSaleRequest{AfterSaleNo: no})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, afterSale)
}

func (h *handlers) approveAfterSale(c *gin.Context) {
	afterSaleNo := strings.TrimSpace(c.Param("no"))
	if afterSaleNo == "" {
		httpx.Error(c, apperr.Invalid("after sale number is required"))
		return
	}
	var in approveRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	approved := true
	if in.Approved != nil {
		approved = *in.Approved
	}
	afterSale, err := h.clients.AfterSales.Approve(c.Request.Context(), &pb.ApproveRequest{
		AfterSaleNo: afterSaleNo,
		Approved:    approved,
		AmountCent:  in.AmountCent,
		Reason:      in.Reason,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, afterSale)
}

// ---------------------------------------------------------------- helpers

// bindJSON decodes the request body into dst and maps decoding failures onto an
// invalid-argument error.
func bindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return apperr.Invalid("invalid request body: %v", err)
	}
	return nil
}

// pathID parses a positive int64 path parameter.
func pathID(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Param(name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Invalid("%s must be a positive integer", name)
	}
	return id, nil
}

// queryInt64 parses an optional int64 query parameter; an absent value is 0.
func queryInt64(c *gin.Context, key string) (int64, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, apperr.Invalid("%s must be an integer", key)
	}
	return value, nil
}

// queryInt32 parses an optional int32 query parameter; an absent value is 0.
func queryInt32(c *gin.Context, key string) (int32, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, apperr.Invalid("%s must be an integer", key)
	}
	return int32(value), nil
}

// remoteError translates a downstream gRPC status into the domain error model
// so httpx.Error emits the right HTTP status. Domain errors pass through.
func remoteError(err error) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperr.Unavailable("downstream call failed", err)
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return apperr.Invalid("%s", st.Message())
	case codes.Unauthenticated:
		return apperr.Unauthenticated(st.Message())
	case codes.PermissionDenied:
		return apperr.Forbidden(st.Message())
	case codes.NotFound:
		return apperr.NotFound(st.Message())
	case codes.AlreadyExists, codes.Aborted:
		return apperr.Conflict(st.Message())
	case codes.FailedPrecondition, codes.OutOfRange:
		return apperr.Failed(st.Message())
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Canceled:
		return apperr.Unavailable("downstream unavailable", err)
	default:
		return apperr.Internal("downstream error", err)
	}
}

func bodyPresent(c *gin.Context) bool {
	return c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0
}
