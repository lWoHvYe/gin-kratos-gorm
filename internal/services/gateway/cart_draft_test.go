package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/auth"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeDraftAI struct {
	pb.AIServiceClient
	completeErr error
	draftJSON   string
	getErr      error
}

func (f *fakeDraftAI) GetCartDraft(context.Context, *pb.GetCartDraftRequest, ...grpc.CallOption) (*pb.CartDraftReply, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	raw := f.draftJSON
	if raw == "" {
		draft, _ := json.Marshal(map[string]any{
			"draft_id": "d1", "items": []map[string]any{{"sku_id": 1, "price_cent": 100, "quantity": 1}},
			"total_cent": 100,
		})
		raw = string(draft)
	}
	return &pb.CartDraftReply{DraftJson: raw}, nil
}
func (f *fakeDraftAI) CompleteCartDraft(context.Context, *pb.CompleteCartDraftRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return nil, f.completeErr
}

type fakeDraftCart struct {
	pb.CartServiceClient
	calls int
}

func (f *fakeDraftCart) ConfirmCartDraft(context.Context, *pb.ConfirmCartDraftRequest, ...grpc.CallOption) (*pb.Cart, error) {
	f.calls++
	return &pb.Cart{UserId: 2, TotalCent: 100, Items: []*pb.CartItem{{SkuId: 1, PriceCent: 100, Quantity: 1}}}, nil
}

type fakeDraftCatalog struct {
	pb.CatalogServiceClient
	price int64
}

func (f *fakeDraftCatalog) GetSku(context.Context, *pb.GetSkuRequest, ...grpc.CallOption) (*pb.Sku, error) {
	price := f.price
	if price == 0 {
		price = 100
	}
	return &pb.Sku{Id: 1, PriceCent: price, Status: "ENABLED"}, nil
}

type fakeDraftInventory struct {
	pb.InventoryServiceClient
	available int64
}

func (f *fakeDraftInventory) BatchGetStock(context.Context, *pb.BatchGetStockRequest, ...grpc.CallOption) (*pb.StockList, error) {
	available := f.available
	if available == 0 {
		available = 5
	}
	return &pb.StockList{Stocks: []*pb.Stock{{SkuId: 1, Available: available}}}, nil
}

func TestConfirmCartDraftReportsUnclosedComplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handlers{
		clients: &Clients{
			AI:   &fakeDraftAI{completeErr: context.DeadlineExceeded},
			Cart: &fakeDraftCart{}, Catalog: &fakeDraftCatalog{}, Inventory: &fakeDraftInventory{},
		},
		log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
	body := []byte(`{"draft_id":"d1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/cart-drafts/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{UserID: 2}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.confirmCartDraft(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var reply map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply["draft_closed"] != false || reply["confirmed"] != true {
		t.Fatalf("reply = %v", reply)
	}
}

func runDraftConfirm(t *testing.T, h *handlers, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/cart-drafts/confirm", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{UserID: 2}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.confirmCartDraft(c)
	var reply map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &reply)
	return w.Code, reply
}

func TestConfirmCartDraftRejectsPriceAndStockChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cart := &fakeDraftCart{}
	h := &handlers{
		clients: &Clients{
			AI: &fakeDraftAI{}, Cart: cart,
			Catalog: &fakeDraftCatalog{price: 200}, Inventory: &fakeDraftInventory{},
		},
		log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
	if code, _ := runDraftConfirm(t, h, `{"draft_id":"d1"}`); code != http.StatusConflict || cart.calls != 0 {
		t.Fatalf("price change: code=%d cart_calls=%d", code, cart.calls)
	}
	h.clients.Catalog = &fakeDraftCatalog{price: 100}
	h.clients.Inventory = &fakeDraftInventory{available: -1}
	if code, _ := runDraftConfirm(t, h, `{"draft_id":"d1"}`); code != http.StatusConflict || cart.calls != 0 {
		t.Fatalf("stock change: code=%d cart_calls=%d", code, cart.calls)
	}
}

func TestConfirmCartDraftReturnsNotFoundForExpiredDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handlers{
		clients: &Clients{
			AI:   &fakeDraftAI{getErr: status.Error(codes.NotFound, "cart draft not found or expired")},
			Cart: &fakeDraftCart{}, Catalog: &fakeDraftCatalog{}, Inventory: &fakeDraftInventory{},
		},
		log: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
	if code, _ := runDraftConfirm(t, h, `{"draft_id":"d1"}`); code != http.StatusNotFound {
		t.Fatalf("expired draft code=%d", code)
	}
}
