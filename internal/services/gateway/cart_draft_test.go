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
)

type fakeDraftAI struct {
	pb.AIServiceClient
	completeErr error
}

func (f *fakeDraftAI) GetCartDraft(context.Context, *pb.GetCartDraftRequest, ...grpc.CallOption) (*pb.CartDraftReply, error) {
	draft, _ := json.Marshal(map[string]any{
		"draft_id": "d1", "items": []map[string]any{{"sku_id": 1, "price_cent": 100, "quantity": 1}},
		"total_cent": 100,
	})
	return &pb.CartDraftReply{DraftJson: string(draft)}, nil
}
func (f *fakeDraftAI) CompleteCartDraft(context.Context, *pb.CompleteCartDraftRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return nil, f.completeErr
}

type fakeDraftCart struct{ pb.CartServiceClient }

func (fakeDraftCart) ConfirmCartDraft(context.Context, *pb.ConfirmCartDraftRequest, ...grpc.CallOption) (*pb.Cart, error) {
	return &pb.Cart{UserId: 2, TotalCent: 100, Items: []*pb.CartItem{{SkuId: 1, PriceCent: 100, Quantity: 1}}}, nil
}

type fakeDraftCatalog struct{ pb.CatalogServiceClient }

func (fakeDraftCatalog) GetSku(context.Context, *pb.GetSkuRequest, ...grpc.CallOption) (*pb.Sku, error) {
	return &pb.Sku{Id: 1, PriceCent: 100, Status: "ENABLED"}, nil
}

type fakeDraftInventory struct{ pb.InventoryServiceClient }

func (fakeDraftInventory) BatchGetStock(context.Context, *pb.BatchGetStockRequest, ...grpc.CallOption) (*pb.StockList, error) {
	return &pb.StockList{Stocks: []*pb.Stock{{SkuId: 1, Available: 5}}}, nil
}

func TestConfirmCartDraftReportsUnclosedComplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handlers{
		clients: &Clients{
			AI:   &fakeDraftAI{completeErr: context.DeadlineExceeded},
			Cart: fakeDraftCart{}, Catalog: fakeDraftCatalog{}, Inventory: fakeDraftInventory{},
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
