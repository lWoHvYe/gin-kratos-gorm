// Package grpcclient provides outbound clients for split deployments.
package grpcclient

import (
	"context"
	"fmt"
	"strings"

	pbProduct "gin-kratos-gorm/api/product/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/apperr"

	etcdregistry "github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ProductCatalog implements biz.ProductCatalog over gRPC.
type ProductCatalog struct {
	client pbProduct.ProductClient
	conn   *grpc.ClientConn
	log    *log.Helper
}

func NewProductCatalog(cfg *conf.Bootstrap, dis *etcdregistry.Registry, logger log.Logger) (*ProductCatalog, func(), error) {
	if cfg == nil || cfg.Services == nil || strings.TrimSpace(cfg.Services.ProductEndpoint) == "" {
		return nil, nil, fmt.Errorf("services.product_endpoint is required by the order service")
	}
	endpoint := strings.TrimSpace(cfg.Services.ProductEndpoint)

	var (
		conn *grpc.ClientConn
		err  error
	)
	if strings.HasPrefix(endpoint, "discovery:///") {
		if dis == nil {
			return nil, nil, fmt.Errorf("services.product_endpoint uses discovery but registry.etcd.endpoints is empty")
		}
		conn, err = kratosgrpc.DialInsecure(
			context.Background(),
			kratosgrpc.WithEndpoint(endpoint),
			kratosgrpc.WithDiscovery(dis),
		)
	} else {
		conn, err = grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		return nil, nil, fmt.Errorf("dial product service %s: %w", endpoint, err)
	}
	cleanup := func() { _ = conn.Close() }
	return &ProductCatalog{client: pbProduct.NewProductClient(conn), conn: conn, log: log.NewHelper(logger)}, cleanup, nil
}

func (c *ProductCatalog) GetSkuSnapshot(ctx context.Context, skuID int64) (*biz.SkuSnapshot, error) {
	reply, err := c.client.GetSku(ctx, &pbProduct.GetSkuReq{SkuId: skuID})
	if err != nil {
		return nil, mapError(err)
	}
	return &biz.SkuSnapshot{
		SkuID:   reply.GetId(),
		SpuID:   reply.GetSpuId(),
		SkuName: reply.GetSkuName(),
		SkuPic:  reply.GetSkuPic(),
		Price:   reply.GetPrice(),
		Stock:   reply.GetStock(),
		OnSale:  reply.GetOnSale(),
	}, nil
}

func (c *ProductCatalog) DeductStock(ctx context.Context, orderSN string, skuID int64, num int32) error {
	_, err := c.client.ReduceStock(ctx, &pbProduct.ReduceStockReq{
		OrderSn: orderSN,
		SkuId:   skuID,
		Num:     num,
	})
	return mapError(err)
}

func (c *ProductCatalog) RestoreStock(ctx context.Context, orderSN string, skuID int64, num int32) error {
	_, err := c.client.CompensateStock(ctx, &pbProduct.ReduceStockReq{
		OrderSn: orderSN,
		SkuId:   skuID,
		Num:     num,
	})
	return mapError(err)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperr.Unavailable("product service call failed", err)
	}
	switch st.Code() {
	case codes.NotFound:
		return apperr.NotFound(st.Message())
	case codes.InvalidArgument:
		return apperr.New(apperr.CodeInvalidArgument, st.Message())
	case codes.FailedPrecondition, codes.Aborted:
		return apperr.FailedPrecondition(st.Message())
	case codes.Unauthenticated:
		return apperr.Unauthenticated(st.Message())
	case codes.PermissionDenied:
		return apperr.Forbidden(st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return apperr.Unavailable("product service unavailable", err)
	default:
		return apperr.Internal("product service error", err)
	}
}
