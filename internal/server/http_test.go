package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pbOrder "gin-kratos-gorm/api/order/v1"
	pbProduct "gin-kratos-gorm/api/product/v1"
	pbUser "gin-kratos-gorm/api/user/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/data/db"
	"gin-kratos-gorm/internal/data/persistence"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/idgen"
	httpServerHandler "gin-kratos-gorm/internal/server/http/handler"
	"gin-kratos-gorm/internal/server/http/router"
	"gin-kratos-gorm/internal/server/middleware"
	"gin-kratos-gorm/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/durationpb"
)

type apiTestEnv struct {
	router  *gin.Engine
	product *persistence.ProductRepo
	users   *persistence.UserRepo
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	source := fmt.Sprintf("file:http_%s?mode=memory&cache=shared", t.Name())
	cfg := &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{Addr: "127.0.0.1:0", Timeout: durationpb.New(5 * time.Second)},
			Grpc: &conf.Server_GRPC{Addr: "127.0.0.1:0", Timeout: durationpb.New(5 * time.Second)},
		},
		Data: &conf.Data{
			Database: &conf.Data_Database{
				Driver: "sqlite", Source: source, MaxOpenConns: 1, AutoMigrate: true, Seed: true,
			},
			Transaction: &conf.Data_Transaction{Mode: "local"},
		},
		Auth: &conf.Auth{
			JwtSecret:      "test-secret-at-least-16-chars",
			JwtIssuer:      "gin-kratos-gorm",
			AccessTokenTtl: durationpb.New(time.Hour),
		},
		Order: &conf.Order{
			PaymentTimeout:        durationpb.New(30 * time.Minute),
			AutoCancelInterval:    durationpb.New(time.Minute),
			ShippingFee:           10,
			FreeShippingThreshold: 99,
		},
		App: &conf.App{Env: "test", MetricsEnabled: true},
	}
	gormDB, err := db.NewDB(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(gormDB) })

	logger := log.NewStdLogger(io.Discard)
	data, cleanup, err := persistence.NewData(cfg, logger, gormDB)
	if err != nil {
		t.Fatalf("new data: %v", err)
	}
	t.Cleanup(cleanup)

	productRepo := persistence.NewProductRepo(data)
	orderRepo := persistence.NewOrderRepo(data)
	userRepo := persistence.NewUserRepo(data)

	jwtManager := auth.NewManager(cfg.Auth)
	userUC := biz.NewUserUsecase(userRepo, logger)
	productUC := biz.NewProductUsecase(productRepo, logger)
	orderUC := biz.NewOrderUsecase(
		orderRepo,
		productRepo,
		&biz.TxSaga{Tx: data, Log: log.NewHelper(logger)},
		biz.RealClock{},
		idgen.NewOrderSN("test"),
		biz.NewOrderConfig(cfg),
		logger,
	)

	userSvc := service.NewUserService(userUC, jwtManager)
	productSvc := service.NewProductService(productUC, productRepo, cfg)
	orderSvc := service.NewOrderService(orderUC, cfg, nil)

	r := newGin(cfg, data, logger)
	public := r.Group("/api")
	private := r.Group("/api")
	private.Use(middleware.JWTAuth(jwtManager))
	admin := private.Group("")
	admin.Use(middleware.RequireRole("admin"))

	router.NewUserRouter(httpServerHandler.NewUserHandler(userSvc)).InitUserRoutes(public, private, admin)
	router.NewProductRouter(httpServerHandler.NewProductHandler(productSvc)).RegisterProductRouter(public, admin)
	router.NewOrderRouter(httpServerHandler.NewOrderHandler(orderSvc)).RegisterRouter(private)

	return &apiTestEnv{router: r, product: productRepo, users: userRepo}
}

func (e *apiTestEnv) request(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *apiTestEnv) login(t *testing.T, name, password string) string {
	t.Helper()
	rec := e.request(t, http.MethodPost, "/api/base/login", "", map[string]string{"name": name, "password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var reply pbUser.UserReply
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if reply.Token == "" {
		t.Fatal("login did not return a token")
	}
	return reply.Token
}

func TestHTTPHealthAndPublicCatalog(t *testing.T) {
	env := newAPITestEnv(t)
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		rec := env.request(t, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	rec := env.request(t, http.MethodGet, "/api/products?page=1&page_size=10", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("products status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list pbProduct.ListProductsReply
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode products: %v", err)
	}
	if list.Total != 2 || len(list.Products) != 2 {
		t.Fatalf("expected seeded products, got total=%d products=%d", list.Total, len(list.Products))
	}
}

func TestHTTPAuthAndOrderLifecycle(t *testing.T) {
	env := newAPITestEnv(t)
	adminToken := env.login(t, "admin", "admin123")
	userToken := env.login(t, "demo", "user123")

	// Unauthenticated order creation is rejected.
	rec := env.request(t, http.MethodPost, "/api/orders", "", map[string]any{
		"receiver_name": "张三", "receiver_address": "上海市",
		"items": []map[string]any{{"sku_id": 1, "num": 1}},
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Non-admin cannot create products.
	rec = env.request(t, http.MethodPost, "/api/products", userToken, map[string]any{
		"name": "forbidden", "skus": []map[string]any{{"sku_code": "X", "price": 1, "stock": 1}},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin product create, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Create an order as the demo user.
	rec = env.request(t, http.MethodPost, "/api/orders", userToken, map[string]any{
		"receiver_name": "张三", "receiver_phone": "13800000000", "receiver_address": "上海市",
		"items": []map[string]any{{"sku_id": 1, "num": 2}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create order status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created pbOrder.CreateOrderReply
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	if created.OrderSn == "" || created.Status != biz.OrderStatusPendingPayment {
		t.Fatalf("unexpected create reply: order_sn=%q status=%d", created.OrderSn, created.Status)
	}

	// Another user must not see the order.
	otherToken := env.login(t, "admin", "admin123")
	_ = otherToken // admin is allowed by design; use a fresh user instead
	rec = env.request(t, http.MethodPost, "/api/base/register", "", map[string]string{
		"name": "charlie", "password": "charlie123", "nickname": "Charlie",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", rec.Code, rec.Body.String())
	}
	var charlie pbUser.UserReply
	_ = json.Unmarshal(rec.Body.Bytes(), &charlie)
	rec = env.request(t, http.MethodGet, "/api/orders/"+created.OrderSn, charlie.Token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for another user's order, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Owner can read, pay and cancel.
	rec = env.request(t, http.MethodGet, "/api/orders/"+created.OrderSn, userToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get order status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = env.request(t, http.MethodPost, "/api/orders/"+created.OrderSn+"/pay", userToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pay status=%d body=%s", rec.Code, rec.Body.String())
	}
	var paid pbOrder.GetOrderDetailReply
	_ = json.Unmarshal(rec.Body.Bytes(), &paid)
	if int32(paid.Status) != biz.OrderStatusPendingShipment {
		t.Fatalf("expected paid status, got %d", paid.Status)
	}
	rec = env.request(t, http.MethodPost, "/api/orders/"+created.OrderSn+"/cancel", adminToken, map[string]string{"reason": "test"})
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Stock is restored (seed sku 1 had 100).
	spu, err := env.product.GetSpu(context.Background(), 1)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if spu.Skus[0].Stock != 100 {
		t.Fatalf("expected stock 100 after cancel, got %d", spu.Skus[0].Stock)
	}
}

func TestHTTPValidationAndAdminProduct(t *testing.T) {
	env := newAPITestEnv(t)
	adminToken := env.login(t, "admin", "admin123")

	rec := env.request(t, http.MethodPost, "/api/orders", adminToken, map[string]any{
		"receiver_name": "张三", "receiver_address": "上海市", "items": []map[string]any{},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty items, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.request(t, http.MethodPost, "/api/orders", adminToken, map[string]any{
		"receiver_name": "张三", "receiver_address": "上海市",
		"items": []map[string]any{{"sku_id": 1, "num": 1000}},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for insufficient stock, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = env.request(t, http.MethodPost, "/api/products", adminToken, map[string]any{
		"name": "New Product", "description": "test", "category_id": 3,
		"skus": []map[string]any{{"sku_code": "NEW-1", "attr_value": "{}", "price": 19.9, "stock": 5}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create product status=%d body=%s", rec.Code, rec.Body.String())
	}
	var product pbProduct.GetProductDetailReply
	_ = json.Unmarshal(rec.Body.Bytes(), &product)
	if product.Id == 0 || len(product.Skus) != 1 {
		t.Fatalf("unexpected created product: id=%d skus=%d", product.Id, len(product.Skus))
	}

	// Adjust stock and verify.
	rec = env.request(t, http.MethodPut, fmt.Sprintf("/api/products/%d/stock", product.Skus[0].Id), adminToken, map[string]any{"delta": 3})
	if rec.Code != http.StatusOK {
		t.Fatalf("adjust stock status=%d body=%s", rec.Code, rec.Body.String())
	}
}
