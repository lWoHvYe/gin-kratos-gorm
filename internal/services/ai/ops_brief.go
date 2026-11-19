package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/redis/go-redis/v9"
)

// OpsBrief is deterministic: every number comes from an aggregate RPC. The
// model is not used, so numeric fidelity cannot drift.
func (s *Service) OpsBrief(ctx context.Context, req *pb.OpsBriefRequest) (*pb.OpsBriefReply, error) {
	days := int(req.GetDays())
	if days <= 0 {
		days = 1
	}
	if days > 30 {
		days = 30
	}
	to := time.Now()
	from := to.AddDate(0, 0, -days)
	orders, err := s.orders.GetOrderStats(ctx, &pb.StatsRequest{FromUnix: from.Unix(), ToUnix: to.Unix()})
	if err != nil {
		return nil, apperr.Unavailable("load order stats", err)
	}
	after, err := s.afterSales.GetAfterSaleStats(ctx, &pb.StatsRequest{FromUnix: from.Unix(), ToUnix: to.Unix()})
	if err != nil {
		return nil, apperr.Unavailable("load after-sale stats", err)
	}
	low, err := s.inventory.ListLowStock(ctx, &pb.ListLowStockRequest{Threshold: 10, Limit: 5})
	if err != nil {
		return nil, apperr.Unavailable("load low stock", err)
	}
	reply := &pb.OpsBriefReply{
		TotalOrders: orders.GetTotalOrders(), PaidOrders: orders.GetPaidOrders(),
		PaidCent: orders.GetPaidCent(), AfterSales: after.GetTotal(),
		RefundedCent: after.GetRefundedCent(), LowStock: low.GetTotal(),
		FromUnix: from.Unix(), ToUnix: to.Unix(), GeneratedAt: time.Now().Unix(),
	}
	for _, stock := range low.GetStocks() {
		reply.ExceptionLinks = append(reply.ExceptionLinks,
			fmt.Sprintf("/admin/inventory?sku_id=%d", stock.GetSkuId()))
	}
	if len(reply.ExceptionLinks) == 0 && low.GetTotal() > 0 {
		reply.ExceptionLinks = append(reply.ExceptionLinks, "/admin/inventory?low_stock=1")
	}
	reply.Summary = fmt.Sprintf(
		"过去 %d 天：订单 %d（已支付 %d，GMV %.2f 元），售后 %d（退款 %.2f 元），低库存 SKU %d。所有数字来自订单/售后/库存聚合接口。",
		days, reply.TotalOrders, reply.PaidOrders, float64(reply.PaidCent)/100,
		reply.AfterSales, float64(reply.RefundedCent)/100, reply.LowStock)
	return reply, nil
}

func (s *Service) LatestOpsBrief(ctx context.Context, _ *pb.Empty) (*pb.OpsBriefReply, error) {
	raw, err := s.cache.Get(ctx, "ai:ops:latest").Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, apperr.NotFound("no scheduled ops brief yet")
		}
		return nil, apperr.Internal("load latest ops brief", err)
	}
	var reply pb.OpsBriefReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, apperr.Internal("decode latest ops brief", err)
	}
	return &reply, nil
}

func (s *Service) OpsFeedback(ctx context.Context, req *pb.OpsFeedbackRequest) (*pb.Empty, error) {
	if req.GetRating() != 1 && req.GetRating() != -1 {
		return nil, apperr.Invalid("rating must be 1 or -1")
	}
	day := strings.TrimSpace(req.GetDay())
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	if len([]rune(req.GetComment())) > 500 {
		return nil, apperr.Invalid("comment must be at most 500 characters")
	}
	entry, _ := json.Marshal(map[string]any{
		"day": day, "rating": req.GetRating(), "comment": strings.TrimSpace(req.GetComment()),
		"admin_user_id": req.GetAdminUserId(), "created_at": time.Now().Unix(),
	})
	key := "ai:ops:feedback:" + day
	if err := s.cache.LPush(ctx, key, entry).Err(); err != nil {
		return nil, apperr.Internal("save ops feedback", err)
	}
	_ = s.cache.LTrim(ctx, key, 0, 999).Err()
	_ = s.cache.Expire(ctx, key, 180*24*time.Hour).Err()
	return &pb.Empty{}, nil
}
