package service

import (
	"context"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/order/v1"
	pbProduct "gin-kratos-gorm/api/product/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/config"

	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DtmCoordinator is the optional remote-saga implementation. It is only wired
// when data.transaction.mode=dtm and the DTM health check succeeds (or the
// operator explicitly allows fallback). The default local mode never touches
// DTM, which keeps local development and integration tests self-contained.
type DtmCoordinator struct {
	cfg *conf.Bootstrap
	uc  *biz.OrderUsecase
	log *log.Helper
}

// NewDtmCoordinator returns (nil, nil) when DTM is disabled or when fallback is
// enabled and the DTM probe fails. A nil coordinator makes OrderService use the
// local compensation saga.
func NewDtmCoordinator(cfg *conf.Bootstrap, uc *biz.OrderUsecase, logger log.Logger) (*DtmCoordinator, error) {
	if !config.DTMEnabled(cfg) {
		return nil, nil
	}
	helper := log.NewHelper(logger)
	address := strings.TrimSpace(cfg.Data.Dtm.Address)
	if address == "" {
		return nil, apperr.Invalid("data.dtm.address is required when DTM is enabled")
	}
	if strings.HasPrefix(address, "discovery:///") && (cfg.Registry == nil || cfg.Registry.Etcd == nil || len(cfg.Registry.Etcd.Endpoints) == 0) {
		if cfg.Data.Transaction.FallbackToLocal {
			helper.Warnf("DTM uses discovery but no etcd endpoints are configured; falling back to local mode")
			return nil, nil
		}
		return nil, apperr.Invalid("DTM discovery address requires registry.etcd.endpoints")
	}
	timeout := 3 * time.Second
	if cfg.Data.Dtm.Timeout != nil && cfg.Data.Dtm.Timeout.AsDuration() > 0 {
		timeout = cfg.Data.Dtm.Timeout.AsDuration()
	}
	if err := probeDTM(address, timeout); err != nil {
		if cfg.Data.Transaction.FallbackToLocal {
			helper.Warnf("DTM is unreachable at %s, falling back to local transaction mode: %v", address, err)
			return nil, nil
		}
		return nil, apperr.Unavailable("DTM is unreachable; enable data.transaction.fallback_to_local or fix the address", err)
	}
	helper.Infof("DTM coordinator enabled, address=%s", address)
	return &DtmCoordinator{cfg: cfg, uc: uc, log: helper}, nil
}

// probeDTM only performs a TCP/HTTP-style gRPC probe for direct addresses.
// Discovery URIs (discovery:///dtmservice) are resolved by the DTM driver and
// are left to the first Submit call.
func probeDTM(address string, timeout time.Duration) error {
	if strings.Contains(address, "://") {
		return nil
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	return conn.Close()
}

func (c *DtmCoordinator) Create(ctx context.Context, in biz.CreateOrderInput, req *pb.CreateOrderReq) (*model.Order, error) {
	quote, err := c.uc.Quote(ctx, in)
	if err != nil {
		return nil, err
	}
	if quote.Existing {
		return quote.Order, nil
	}

	order := quote.Order
	gid := "order-" + order.OrderSn
	saga := dtmgrpc.NewSagaGrpc(c.cfg.Data.Dtm.Address, gid)
	saga.WaitResult = true
	saga.TimeoutToFail = 30
	saga.RetryLimit = 3

	for _, item := range quote.Items {
		payload := &pbProduct.ReduceStockReq{
			OrderSn: order.OrderSn,
			SkuId:   item.SkuID,
			Num:     item.Num,
		}
		saga.Add(
			"discovery:///product"+pbProduct.Product_ReduceStock_FullMethodName,
			"discovery:///product"+pbProduct.Product_CompensateStock_FullMethodName,
			payload,
		)
	}

	inner := &pb.CreateOrderReq{
		UserId:          uint64(in.UserID),
		OrderSn:         order.OrderSn,
		Remark:          in.Remark,
		Items:           req.GetItems(),
		ClientRequestId: in.ClientRequestID,
		ReceiverName:    in.ReceiverName,
		ReceiverPhone:   in.ReceiverPhone,
		ReceiverAddress: in.ReceiverAddress,
		TotalAmount:     order.TotalAmount,
		FreightAmount:   order.FreightAmount,
		PayAmount:       order.PayAmount,
	}
	if order.ExpireAt != nil {
		inner.ExpireAtUnix = order.ExpireAt.Unix()
	}
	saga.Add(
		"discovery:///order"+pb.Order_CreateOrderInner_FullMethodName,
		"discovery:///order"+pb.Order_CancelOrder_FullMethodName,
		inner,
	)

	if err := saga.Submit(); err != nil {
		c.log.Errorf("DTM saga submit failed (gid=%s): %v", gid, err)
		return nil, apperr.Unavailable("distributed transaction failed", err)
	}
	// WaitResult=true means the saga completed (or compensated) before Submit
	// returns. Re-read the order so callers get the persisted status.
	if persisted, getErr := c.uc.Get(ctx, 0, order.OrderSn, true); getErr == nil {
		return persisted, nil
	}
	return order, nil
}
