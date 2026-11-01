// Package gateway implements the public HTTP BFF of the mall. It owns no
// persistence: requests are authenticated locally with the shared JWT secret,
// mapped onto the generated protobuf request types, and forwarded to the
// service that owns the data.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/grpcx"
	"gin-kratos-gorm/internal/platform/httpx"
	"gin-kratos-gorm/internal/platform/id"

	"github.com/gin-gonic/gin"
	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// requestTimeout bounds one BFF request, including every downstream call.
	requestTimeout = 15 * time.Second
	// defaultPaymentChannel is used when the client omits "channel" on the
	// simulated payment callback.
	defaultPaymentChannel = "MOCK"
	// paymentSucceeded is the only callback status exposed by this route.
	paymentSucceeded = "SUCCEEDED"
)

// serviceAddrs are the direct gRPC addresses used when no registry is
// configured. They mirror the port table in docs/contracts.md and can be
// overridden with <SERVICE>_ADDR (e.g. IDENTITY_ADDR).
var serviceAddrs = map[string]string{
	"identity":    "127.0.0.1:9001",
	"catalog":     "127.0.0.1:9002",
	"cart":        "127.0.0.1:9004",
	"pricing":     "127.0.0.1:9005",
	"order":       "127.0.0.1:9006",
	"payment":     "127.0.0.1:9007",
	"fulfillment": "127.0.0.1:9008",
	"aftersales":  "127.0.0.1:9009",
}

// Clients bundles every downstream gRPC client used by the public BFF.
type Clients struct {
	Identity    pb.IdentityServiceClient
	Catalog     pb.CatalogServiceClient
	Cart        pb.CartServiceClient
	Pricing     pb.PricingServiceClient
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

// Dial opens the downstream client connections. With a registry the clients use
// discovery:///<service> targets; without one they fall back to the direct
// addresses from <SERVICE>_ADDR.
func Dial(ctx context.Context, reg *etcdregistry.Registry) (*Clients, error) {
	clients := &Clients{}
	targets := []struct {
		name string
		bind func(*grpc.ClientConn)
	}{
		{"identity", func(conn *grpc.ClientConn) { clients.Identity = pb.NewIdentityServiceClient(conn) }},
		{"catalog", func(conn *grpc.ClientConn) { clients.Catalog = pb.NewCatalogServiceClient(conn) }},
		{"cart", func(conn *grpc.ClientConn) { clients.Cart = pb.NewCartServiceClient(conn) }},
		{"pricing", func(conn *grpc.ClientConn) { clients.Pricing = pb.NewPricingServiceClient(conn) }},
		{"order", func(conn *grpc.ClientConn) { clients.Order = pb.NewOrderServiceClient(conn) }},
		{"payment", func(conn *grpc.ClientConn) { clients.Payment = pb.NewPaymentServiceClient(conn) }},
		{"fulfillment", func(conn *grpc.ClientConn) { clients.Fulfillment = pb.NewFulfillmentServiceClient(conn) }},
		{"aftersales", func(conn *grpc.ClientConn) { clients.AfterSales = pb.NewAfterSalesServiceClient(conn) }},
	}
	for _, target := range targets {
		conn, err := grpcx.Dial(ctx, targetFor(reg, target.name), reg)
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
func targetFor(reg *etcdregistry.Registry, name string) string {
	if reg != nil {
		return "discovery:///" + name
	}
	if addr := strings.TrimSpace(os.Getenv(strings.ToUpper(name) + "_ADDR")); addr != "" {
		return addr
	}
	if addr, ok := serviceAddrs[name]; ok {
		return addr
	}
	return name + ":9000"
}

// NewEngine builds the public gateway Gin engine. The routes under /api/v1 that
// are not explicitly public require a valid bearer token.
func NewEngine(logger *slog.Logger, jwt *auth.Manager, clients *Clients) *gin.Engine {
	if logger == nil {
		logger = slog.Default()
	}
	h := &handlers{clients: clients}

	engine := gin.New()
	engine.Use(recovery(), httpx.RequestID(), httpx.AccessLog(logger), httpx.CORS(), timeout(requestTimeout))
	engine.NoRoute(func(c *gin.Context) { httpx.Error(c, apperr.NotFound("route not found")) })
	engine.GET("/healthz", h.health)

	engine.POST("/api/v1/auth/register", h.register)
	engine.POST("/api/v1/auth/login", h.login)
	engine.POST("/api/v1/auth/refresh", h.refresh)
	engine.POST("/api/v1/auth/logout", h.logout)
	engine.GET("/api/v1/categories", h.listCategories)
	engine.GET("/api/v1/products", h.listProducts)
	engine.GET("/api/v1/products/:id", h.getProduct)
	engine.GET("/api/v1/skus/:id", h.getSku)

	authed := engine.Group("/api/v1", httpx.Auth(jwt), requireUser())
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
	authed.POST("/payments/:paymentNo/pay", h.pay)
	authed.POST("/after-sales", h.createAfterSale)
	authed.GET("/after-sales/:no", h.getAfterSale)

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

// requireUser rejects unauthenticated requests before they reach a handler.
func requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := userID(c); err != nil {
			httpx.Error(c, err)
			return
		}
		c.Next()
	}
}

// ---------------------------------------------------------------- public

func (h *handlers) health(c *gin.Context) { httpx.OK(c, gin.H{"status": "ok"}) }

func (h *handlers) register(c *gin.Context) {
	var req pb.RegisterRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Register(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.Created(c, reply)
}

func (h *handlers) login(c *gin.Context) {
	var req pb.LoginRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Login(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *handlers) refresh(c *gin.Context) {
	var in refreshRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Identity.Refresh(c.Request.Context(), &pb.RefreshRequest{RefreshToken: in.RefreshToken})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) logout(c *gin.Context) {
	var in refreshRequest
	if bodyPresent(c) {
		if err := bindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	if _, err := h.clients.Identity.Logout(c.Request.Context(), &pb.LogoutRequest{RefreshToken: in.RefreshToken}); err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": "logged out"})
}

func (h *handlers) listCategories(c *gin.Context) {
	reply, err := h.clients.Catalog.ListCategories(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) listProducts(c *gin.Context) {
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
	categoryID, err := queryInt64(c, "category_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Catalog.ListProducts(c.Request.Context(), &pb.ListProductsRequest{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    strings.TrimSpace(c.Query("keyword")),
		CategoryId: categoryID,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) getProduct(c *gin.Context) {
	productID, err := pathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	product, err := h.clients.Catalog.GetProduct(c.Request.Context(), &pb.GetProductRequest{Id: productID})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, product)
}

func (h *handlers) getSku(c *gin.Context) {
	skuID, err := pathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	sku, err := h.clients.Catalog.GetSku(c.Request.Context(), &pb.GetSkuRequest{SkuId: skuID})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, sku)
}

// ---------------------------------------------------------------- cart

func (h *handlers) getCart(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	cart, err := h.clients.Cart.GetCart(c.Request.Context(), &pb.GetCartRequest{UserId: uid})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) addCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.AddItemRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.UserId = uid
	cart, err := h.clients.Cart.AddItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) updateCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.UpdateItemRequest
	if err := bindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	req.UserId = uid
	cart, err := h.clients.Cart.UpdateItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, cart)
}

func (h *handlers) removeCartItem(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req pb.RemoveItemRequest
	if bodyPresent(c) {
		if err := bindJSON(c, &req); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	if req.SkuId == 0 {
		// DELETE requests often carry no body; allow ?sku_id=<id> as well.
		skuID, err := queryInt64(c, "sku_id")
		if err != nil {
			httpx.Error(c, err)
			return
		}
		req.SkuId = skuID
	}
	req.UserId = uid
	cart, err := h.clients.Cart.RemoveItem(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, cart)
}

// mergeRequest is the body of POST /cart/merge. A client sends the anonymous
// cart; every line is added to the authenticated user's cart.
type mergeRequest struct {
	Items []mergeItem `json:"items"`
}

type mergeItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) mergeCart(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in mergeRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	for _, item := range in.Items {
		if item.SkuID <= 0 || item.Quantity <= 0 {
			httpx.Error(c, apperr.Invalid("merge items require a positive sku_id and quantity"))
			return
		}
	}
	var cart *pb.Cart
	for _, item := range in.Items {
		cart, err = h.clients.Cart.AddItem(c.Request.Context(), &pb.AddItemRequest{
			UserId:   uid,
			SkuId:    item.SkuID,
			Quantity: item.Quantity,
		})
		if err != nil {
			httpx.Error(c, remoteError(err))
			return
		}
	}
	if cart == nil {
		cart, err = h.clients.Cart.GetCart(c.Request.Context(), &pb.GetCartRequest{UserId: uid})
		if err != nil {
			httpx.Error(c, remoteError(err))
			return
		}
	}
	httpx.OK(c, cart)
}

// ---------------------------------------------------------------- checkout

// previewRequest is the body of POST /checkout/preview. The pb.QuoteRequest
// user_id is always replaced with the authenticated user.
type previewRequest struct {
	Items     []checkoutItem `json:"items"`
	AddressID int64          `json:"address_id"`
	CouponID  string         `json:"coupon_id"`
}

type checkoutItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) previewCheckout(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in previewRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if len(in.Items) == 0 {
		httpx.Error(c, apperr.Invalid("items are required"))
		return
	}
	items := make([]*pb.QuoteItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.QuoteItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	quote, err := h.clients.Pricing.CreateQuote(c.Request.Context(), &pb.QuoteRequest{
		UserId:    uid,
		Items:     items,
		AddressId: in.AddressID,
		CouponId:  strings.TrimSpace(in.CouponID),
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, quote)
}

// placeOrderRequest is the body of POST /checkout/place. The order service
// rebuilds the lines from the quote, so only the quote and receiver are needed.
type placeOrderRequest struct {
	QuoteID         string `json:"quote_id"`
	AddressID       int64  `json:"address_id"`
	ReceiverName    string `json:"receiver_name"`
	ReceiverPhone   string `json:"receiver_phone"`
	ReceiverAddress string `json:"receiver_address"`
	ClientRequestID string `json:"client_request_id"`
}

func (h *handlers) placeOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in placeOrderRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	clientRequestID := strings.TrimSpace(in.ClientRequestID)
	if clientRequestID == "" {
		clientRequestID = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if clientRequestID == "" {
		clientRequestID = id.Event()
	}
	order, err := h.clients.Order.CreateOrder(c.Request.Context(), &pb.CreateOrderRequest{
		UserId:          uid,
		QuoteId:         strings.TrimSpace(in.QuoteID),
		AddressId:       in.AddressID,
		ClientRequestId: clientRequestID,
		ReceiverName:    strings.TrimSpace(in.ReceiverName),
		ReceiverPhone:   strings.TrimSpace(in.ReceiverPhone),
		ReceiverAddress: strings.TrimSpace(in.ReceiverAddress),
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	intent, err := h.clients.Payment.CreateIntent(c.Request.Context(), &pb.CreateIntentRequest{
		OrderNo:    order.GetOrderNo(),
		UserId:     uid,
		AmountCent: order.GetPayCent(),
		Channel:    defaultPaymentChannel,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.Created(c, gin.H{"order": order, "payment": intent})
}

// ---------------------------------------------------------------- orders

func (h *handlers) listOrders(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
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
	orders, err := h.clients.Order.ListOrders(c.Request.Context(), &pb.ListOrdersRequest{
		UserId:   uid,
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

func (h *handlers) getOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	order, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, order)
}

func (h *handlers) cancelOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	req := &pb.CancelOrderRequest{OrderNo: orderNo, UserId: uid}
	if bodyPresent(c) {
		var in pb.CancelOrderRequest
		if err := bindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
		req.Reason = in.GetReason()
	}
	order, err := h.clients.Order.CancelOrder(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, order)
}

// receiveRequest is the optional body of POST /orders/:orderNo/receive. The
// shipment is resolved by order number when shipment_no is omitted.
type receiveRequest struct {
	ShipmentNo string `json:"shipment_no"`
}

func (h *handlers) receiveOrder(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("orderNo is required"))
		return
	}
	// Ownership check before touching another service's shipment.
	if _, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid}); err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	var in receiveRequest
	if bodyPresent(c) {
		if err := bindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	shipmentNo := strings.TrimSpace(in.ShipmentNo)
	if shipmentNo == "" {
		shipmentNo = strings.TrimSpace(c.Query("shipment_no"))
	}
	var shipment *pb.Shipment
	if shipmentNo == "" {
		shipment, err = h.clients.Fulfillment.GetShipmentByOrder(c.Request.Context(), &pb.GetShipmentByOrderRequest{OrderNo: orderNo})
	} else {
		shipment, err = h.clients.Fulfillment.GetShipment(c.Request.Context(), &pb.GetShipmentRequest{ShipmentNo: shipmentNo})
	}
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	if shipment.GetOrderNo() != orderNo {
		httpx.Error(c, apperr.Forbidden("shipment does not belong to this order"))
		return
	}
	shipmentNo = shipment.GetShipmentNo()
	received, err := h.clients.Fulfillment.Receive(c.Request.Context(), &pb.ReceiveRequest{ShipmentNo: shipmentNo})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, received)
}

// ---------------------------------------------------------------- payment

// payRequest is the body of POST /payments/:paymentNo/pay. It simulates a
// channel callback; the status is always SUCCEEDED.
type payRequest struct {
	Channel    string `json:"channel"`
	EventID    string `json:"event_id"`
	AmountCent int64  `json:"amount_cent"`
	Signature  string `json:"signature"`
}

func (h *handlers) pay(c *gin.Context) {
	paymentNo := strings.TrimSpace(c.Param("paymentNo"))
	if paymentNo == "" {
		httpx.Error(c, apperr.Invalid("paymentNo is required"))
		return
	}
	var in payRequest
	if bodyPresent(c) {
		if err := bindJSON(c, &in); err != nil {
			httpx.Error(c, err)
			return
		}
	}
	eventID := strings.TrimSpace(in.EventID)
	if eventID == "" {
		eventID = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	if eventID == "" {
		eventID = id.Event()
	}
	channel := strings.TrimSpace(in.Channel)
	if channel == "" {
		channel = defaultPaymentChannel
	}
	// The simulated pay endpoint only receives payment_no. Load the intent so
	// the callback carries the authoritative amount and ownership can be
	// checked against the order.
	intent, err := h.clients.Payment.GetIntent(c.Request.Context(), &pb.GetIntentRequest{PaymentNo: paymentNo})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if intent.GetOrderNo() != "" {
		order, oerr := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: intent.GetOrderNo(), UserId: uid})
		if oerr != nil {
			httpx.Error(c, remoteError(oerr))
			return
		}
		if order.GetUserId() != uid {
			httpx.Error(c, apperr.Forbidden("payment belongs to another user"))
			return
		}
	}
	amount := in.AmountCent
	if amount <= 0 {
		amount = intent.GetAmountCent()
	}
	reply, err := h.clients.Payment.Callback(c.Request.Context(), &pb.CallbackRequest{
		PaymentNo:  paymentNo,
		Channel:    channel,
		EventId:    eventID,
		AmountCent: amount,
		Status:     paymentSucceeded,
		Signature:  in.Signature,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.OK(c, reply)
}

// ---------------------------------------------------------------- after sales

// createAfterSaleRequest is the body of POST /after-sales. user_id is always
// taken from the token.
type createAfterSaleRequest struct {
	OrderNo    string          `json:"order_no"`
	Type       string          `json:"type"`
	Reason     string          `json:"reason"`
	AmountCent int64           `json:"amount_cent"`
	Items      []afterSaleItem `json:"items"`
}

type afterSaleItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

func (h *handlers) createAfterSale(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in createAfterSaleRequest
	if err := bindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	items := make([]*pb.AfterSaleItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, &pb.AfterSaleItem{SkuId: item.SkuID, Quantity: item.Quantity})
	}
	orderNo := strings.TrimSpace(in.OrderNo)
	if orderNo == "" {
		httpx.Error(c, apperr.Invalid("order_no is required"))
		return
	}
	// The after-sales service does not consult orders, so ownership of the
	// order is checked here before a request can be opened against it.
	if _, err := h.clients.Order.GetOrder(c.Request.Context(), &pb.GetOrderRequest{OrderNo: orderNo, UserId: uid}); err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	afterSale, err := h.clients.AfterSales.CreateAfterSale(c.Request.Context(), &pb.CreateAfterSaleRequest{
		OrderNo:    orderNo,
		UserId:     uid,
		Type:       strings.TrimSpace(in.Type),
		Reason:     in.Reason,
		AmountCent: in.AmountCent,
		Items:      items,
	})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	httpx.Created(c, afterSale)
}

func (h *handlers) getAfterSale(c *gin.Context) {
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	afterSaleNo := strings.TrimSpace(c.Param("no"))
	if afterSaleNo == "" {
		httpx.Error(c, apperr.Invalid("after sale number is required"))
		return
	}
	afterSale, err := h.clients.AfterSales.GetAfterSale(c.Request.Context(), &pb.GetAfterSaleRequest{AfterSaleNo: afterSaleNo})
	if err != nil {
		httpx.Error(c, remoteError(err))
		return
	}
	if afterSale.GetUserId() != uid {
		httpx.Error(c, apperr.Forbidden("after sale belongs to another user"))
		return
	}
	httpx.OK(c, afterSale)
}

// ---------------------------------------------------------------- helpers

// userID returns the authenticated user. The auth middleware stores the claims
// in the request context; a missing or zero id is rejected.
func userID(c *gin.Context) (int64, error) {
	uid, ok := auth.UserID(c.Request.Context())
	if !ok || uid == 0 {
		return 0, apperr.Unauthenticated("authentication required")
	}
	if uid > math.MaxInt64 {
		return 0, apperr.Unauthenticated("invalid user id")
	}
	return int64(uid), nil
}

// bindJSON decodes the request body into dst and maps decoding failures onto an
// invalid-argument error.
func bindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return apperr.Invalid("invalid request body: %v", err)
	}
	return nil
}

// bodyPresent reports whether the request carries a body.
func bodyPresent(c *gin.Context) bool {
	return c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0
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
