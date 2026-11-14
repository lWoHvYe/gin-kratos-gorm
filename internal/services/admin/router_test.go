package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/config"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type fakeOrderClient struct {
	pb.OrderServiceClient
	gotAdmin bool
	gotOrder string
}

func (f *fakeOrderClient) GetOrder(ctx context.Context, in *pb.GetOrderRequest, _ ...grpc.CallOption) (*pb.Order, error) {
	f.gotAdmin = in.GetAdmin()
	f.gotOrder = in.GetOrderNo()
	return &pb.Order{OrderNo: in.GetOrderNo(), UserId: 7, Status: "PENDING_PAYMENT"}, nil
}

func testManager(t *testing.T) *auth.Manager {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return auth.NewManager(&config.Config{
		JWTSecret:  "0123456789abcdef0123456789abcdef",
		JWTIssuer:  "mall",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
}

func TestAdminOrderDetailRoute(t *testing.T) {
	manager := testManager(t)
	fake := &fakeOrderClient{}
	engine := NewEngine(nil, manager, &Clients{Order: fake})
	token, _, err := manager.Generate(1, []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/orders/ORD-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !fake.gotAdmin || fake.gotOrder != "ORD-1" {
		t.Fatalf("order call = admin:%v no:%q", fake.gotAdmin, fake.gotOrder)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// Zero values must be present or the admin UI renders blanks/NaN.
	if v, ok := body["refund_status"]; !ok || v != "" {
		t.Fatalf("refund_status = %#v (present=%v)", v, ok)
	}
	if v, ok := body["paid_cent"]; !ok || v != float64(0) {
		t.Fatalf("paid_cent = %#v (present=%v)", v, ok)
	}
}

func TestAdminOrderDetailRequiresAdmin(t *testing.T) {
	manager := testManager(t)
	engine := NewEngine(nil, manager, &Clients{})
	token, _, err := manager.Generate(2, []string{"user"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/orders/ORD-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAdminRouteRequiresToken(t *testing.T) {
	manager := testManager(t)
	engine := NewEngine(nil, manager, &Clients{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/orders/ORD-1", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
