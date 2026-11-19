package ai

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/cloudwego/eino/schema"
	"google.golang.org/grpc"
)

type fakeCatalog struct {
	keyword string
}

func (f *fakeCatalog) ListProducts(_ context.Context, in *pb.ListProductsRequest, _ ...grpc.CallOption) (*pb.ProductList, error) {
	f.keyword = in.GetKeyword()
	return &pb.ProductList{
		Total: 1,
		Products: []*pb.Product{{
			Id:           7,
			Name:         "静音键盘",
			Subtitle:     "适合办公室",
			MinPriceCent: 29900,
			Description:  "低噪音机械键盘，支持热插拔。",
			Attributes:   []*pb.ProductAttribute{{Name: "轴体", Value: "静音红轴"}},
		}},
	}, nil
}

func (f *fakeCatalog) GetProduct(_ context.Context, in *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	return &pb.Product{Id: in.GetId(), Name: "静音键盘", MinPriceCent: 29900}, nil
}

func (f *fakeCatalog) GetSku(_ context.Context, in *pb.GetSkuRequest, _ ...grpc.CallOption) (*pb.Sku, error) {
	return &pb.Sku{Id: in.GetSkuId(), SpuId: 7, PriceCent: 29900, Status: "ENABLED"}, nil
}

func TestMockAgentCallsCatalogTool(t *testing.T) {
	catalog := &fakeCatalog{}
	ag, err := buildADKAgent(context.Background(), ProviderConfig{Provider: "mock"}, Deps{Catalog: catalog}, false)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := ag.Stream(context.Background(), []*schema.Message{{Role: schema.User, Content: "推荐键盘"}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var answer strings.Builder
	for {
		msg, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatal(recvErr)
		}
		answer.WriteString(msg.Content)
	}
	if catalog.keyword != "推荐键盘" {
		t.Fatalf("tool keyword = %q", catalog.keyword)
	}
	if !strings.Contains(answer.String(), "静音键盘") || !strings.Contains(answer.String(), "299.00") {
		t.Fatalf("answer = %q", answer.String())
	}
}
