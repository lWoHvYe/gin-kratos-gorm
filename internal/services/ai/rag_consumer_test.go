package ai

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

type grpcCallOption = grpc.CallOption

type fakeIndexer struct {
	upserts int
	deletes int
}

func (f *fakeIndexer) UpsertProduct(context.Context, *pb.Product) error { f.upserts++; return nil }
func (f *fakeIndexer) DeleteProduct(context.Context, int64) error       { f.deletes++; return nil }

type indexCatalog struct {
	product *pb.Product
	err     error
}

func (c indexCatalog) ListProducts(context.Context, *pb.ListProductsRequest, ...grpcCallOption) (*pb.ProductList, error) {
	return &pb.ProductList{}, nil
}
func (c indexCatalog) GetProduct(context.Context, *pb.GetProductRequest, ...grpcCallOption) (*pb.Product, error) {
	return c.product, c.err
}
func (c indexCatalog) GetSku(context.Context, *pb.GetSkuRequest, ...grpcCallOption) (*pb.Sku, error) {
	return &pb.Sku{}, c.err
}

func TestApplyCatalogChangeUpsertsAndDeletes(t *testing.T) {
	idx := &fakeIndexer{}
	catalog := indexCatalog{product: &pb.Product{Id: 1000, Status: "ON_SALE"}}
	if err := applyCatalogChange(context.Background(), catalog, idx, catalogChangedEvent{SpuID: 1000, Status: "ON_SALE"}); err != nil {
		t.Fatal(err)
	}
	if idx.upserts != 1 || idx.deletes != 0 {
		t.Fatalf("upserts=%d deletes=%d", idx.upserts, idx.deletes)
	}

	if err := applyCatalogChange(context.Background(), catalog, idx, catalogChangedEvent{SpuID: 1000, Status: "OFF_SALE"}); err != nil {
		t.Fatal(err)
	}
	if idx.deletes != 1 {
		t.Fatalf("deletes=%d", idx.deletes)
	}

	catalog = indexCatalog{err: apperr.NotFound("gone")}
	if err := applyCatalogChange(context.Background(), catalog, idx, catalogChangedEvent{SpuID: 1000, Status: "ON_SALE"}); err != nil {
		t.Fatal(err)
	}
	if idx.deletes != 2 {
		t.Fatalf("deletes=%d", idx.deletes)
	}
}

func TestDecodeCatalogEventAcceptsWrappedPayload(t *testing.T) {
	ev, err := decodeCatalogEvent([]byte(`"{\"spu_id\":1000,\"status\":\"ON_SALE\"}"`))
	if err != nil || ev.SpuID != 1000 || ev.Status != "ON_SALE" {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
}
