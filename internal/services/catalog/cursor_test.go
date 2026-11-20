package catalog

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestProductCursorRoundTrip(t *testing.T) {
	raw := encodeProductCursor("price_asc", 29900, 42)
	c, err := decodeProductCursor(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sort != "price_asc" || c.Price != 29900 || c.ID != 42 {
		t.Fatalf("cursor = %+v", c)
	}
	if _, err := decodeProductCursor("not-a-cursor"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}

func TestListProductsRejectsPageOverLimit(t *testing.T) {
	s := &Service{}
	if _, err := s.ListProducts(context.Background(), &pb.ListProductsRequest{Page: 1001}); err == nil {
		t.Fatal("page > 1000 must be rejected")
	}
}
