package service

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/order/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"
)

type OrderService struct {
	pb.UnimplementedOrderServer
	uc  *biz.OrderUsecase
	cfg *conf.Bootstrap
	dtm *DtmCoordinator
}

func NewOrderService(uc *biz.OrderUsecase, cfg *conf.Bootstrap, dtm *DtmCoordinator) *OrderService {
	return &OrderService{uc: uc, cfg: cfg, dtm: dtm}
}

func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderReq) (*pb.CreateOrderReply, error) {
	userID := int64(req.GetUserId())
	if id, ok := auth.UserID(ctx); ok {
		userID = int64(id)
	}
	if userID <= 0 {
		return nil, apperr.Unauthenticated("login required")
	}
	in := createOrderInput(req, userID)
	if s.dtm != nil {
		order, err := s.dtm.Create(ctx, in, req)
		if err != nil {
			return nil, err
		}
		return toCreateReply(order), nil
	}
	order, err := s.uc.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return toCreateReply(order), nil
}

// CreateOrderInner is the DTM branch endpoint. Stock has already been deducted
// by the DTM saga, so it must not deduct again.
func (s *OrderService) CreateOrderInner(ctx context.Context, req *pb.CreateOrderReq) (*pb.CreateOrderReply, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	pre := &biz.PrecomputedAmounts{
		TotalAmount:   req.GetTotalAmount(),
		FreightAmount: req.GetFreightAmount(),
		PayAmount:     req.GetPayAmount(),
	}
	order, err := s.uc.CreateInner(ctx, createOrderInput(req, int64(req.GetUserId())), pre)
	if err != nil {
		return nil, err
	}
	return toCreateReply(order), nil
}

// CancelOrder is the DTM compensation endpoint. Its payload type matches the
// action payload (CreateOrderReq) as required by DTM.
func (s *OrderService) CancelOrder(ctx context.Context, req *pb.CreateOrderReq) (*pb.CreateOrderReply, error) {
	order, err := s.uc.Cancel(ctx, 0, req.GetOrderSn(), "dtm compensation", true)
	if err != nil {
		// Compensation is idempotent: if the action never created the order,
		// there is nothing to undo.
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return &pb.CreateOrderReply{}, nil
		}
		return nil, err
	}
	return toCreateReply(order), nil
}

func (s *OrderService) CancelOrderByUser(ctx context.Context, req *pb.CancelOrderReq) (*pb.GetOrderDetailReply, error) {
	userID, admin, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	order, err := s.uc.Cancel(ctx, userID, req.GetOrderSn(), req.GetReason(), admin)
	if err != nil {
		return nil, err
	}
	return toOrderDetail(order), nil
}

func (s *OrderService) PayOrder(ctx context.Context, req *pb.PayOrderReq) (*pb.GetOrderDetailReply, error) {
	userID, admin, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	order, err := s.uc.Pay(ctx, userID, req.GetOrderSn(), admin)
	if err != nil {
		return nil, err
	}
	return toOrderDetail(order), nil
}

func (s *OrderService) GetOrderDetail(ctx context.Context, req *pb.GetOrderDetailReq) (*pb.GetOrderDetailReply, error) {
	userID, admin, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	order, err := s.uc.Get(ctx, userID, req.GetOrderSn(), admin)
	if err != nil {
		return nil, err
	}
	return toOrderDetail(order), nil
}

func (s *OrderService) ListOrders(ctx context.Context, req *pb.ListOrdersReq) (*pb.ListOrdersReply, error) {
	userID, admin, err := s.identity(ctx)
	if err != nil {
		return nil, err
	}
	filter := biz.OrderFilter{
		Status:   req.Status,
		OrderSN:  req.GetOrderSn(),
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	}
	orders, total, err := s.uc.List(ctx, userID, filter, admin)
	if err != nil {
		return nil, err
	}
	reply := &pb.ListOrdersReply{
		Total:    total,
		Page:     int32(filter.Page),
		PageSize: int32(filter.PageSize),
	}
	for _, order := range orders {
		reply.Orders = append(reply.Orders, toOrderSummary(order))
	}
	return reply, nil
}

func (s *OrderService) identity(ctx context.Context) (int64, bool, error) {
	userID, ok := auth.UserID(ctx)
	if !ok || userID == 0 {
		return 0, false, apperr.Unauthenticated("login required")
	}
	return int64(userID), auth.HasRole(ctx, "admin"), nil
}

func createOrderInput(req *pb.CreateOrderReq, userID int64) biz.CreateOrderInput {
	in := biz.CreateOrderInput{
		UserID:          userID,
		OrderSN:         req.GetOrderSn(),
		ReceiverName:    req.GetReceiverName(),
		ReceiverPhone:   req.GetReceiverPhone(),
		ReceiverAddress: req.GetReceiverAddress(),
		Remark:          req.GetRemark(),
		ClientRequestID: req.GetClientRequestId(),
	}
	for _, item := range req.GetItems() {
		in.Items = append(in.Items, biz.CreateOrderItem{SkuID: int64(item.GetSkuId()), Num: item.GetNum()})
	}
	return in
}

func toCreateReply(order *model.Order) *pb.CreateOrderReply {
	reply := &pb.CreateOrderReply{
		OrderSn:       order.OrderSn,
		TotalAmount:   order.TotalAmount,
		Status:        order.Status,
		PayAmount:     order.PayAmount,
		FreightAmount: order.FreightAmount,
	}
	if order.ExpireAt != nil {
		reply.ExpireAt = order.ExpireAt.Format(time.RFC3339)
	}
	return reply
}

func toOrderDetail(order *model.Order) *pb.GetOrderDetailReply {
	reply := &pb.GetOrderDetailReply{
		OrderSn:         order.OrderSn,
		Status:          uint32(order.Status),
		StatusText:      biz.OrderStatusText(order.Status),
		TotalAmount:     order.TotalAmount,
		PayAmount:       order.PayAmount,
		FreightAmount:   order.FreightAmount,
		ReceiverName:    order.ReceiverName,
		ReceiverPhone:   order.ReceiverPhone,
		ReceiverAddress: order.ReceiverAddress,
		Remark:          order.Remark,
		CancelReason:    order.CancelReason,
	}
	if !order.CreatedAt.IsZero() {
		reply.CreatedAt = order.CreatedAt.Format(time.RFC3339)
	}
	if order.PayTime != nil {
		reply.PayTime = order.PayTime.Format(time.RFC3339)
	}
	if order.ExpireAt != nil {
		reply.ExpireAt = order.ExpireAt.Format(time.RFC3339)
	}
	for _, item := range order.OrderItems {
		reply.Items = append(reply.Items, &pb.OrderItemInfo{
			SkuId:   uint32(item.SkuID),
			SkuName: item.SkuName,
			SkuPic:  item.SkuPic,
			Price:   item.Price,
			Num:     item.Num,
		})
	}
	return reply
}

func toOrderSummary(order *model.Order) *pb.OrderSummary {
	summary := &pb.OrderSummary{
		OrderSn:      order.OrderSn,
		Status:       uint32(order.Status),
		StatusText:   biz.OrderStatusText(order.Status),
		PayAmount:    order.PayAmount,
		ReceiverName: order.ReceiverName,
	}
	if !order.CreatedAt.IsZero() {
		summary.CreatedAt = order.CreatedAt.Format(time.RFC3339)
	}
	for _, item := range order.OrderItems {
		summary.Items = append(summary.Items, &pb.OrderItemInfo{
			SkuId:   uint32(item.SkuID),
			SkuName: item.SkuName,
			SkuPic:  item.SkuPic,
			Price:   item.Price,
			Num:     item.Num,
		})
	}
	return summary
}
