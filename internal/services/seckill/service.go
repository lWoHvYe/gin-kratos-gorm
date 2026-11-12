package seckill

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/redis/go-redis/v9"
)

const (
	// warmupTTL keeps the admission keys alive for a short safety window after
	// the activity ends. Seckill never resurrects stock from an expired key.
	warmupTTL = 2 * time.Hour
)

// Service owns seckill activity metadata. The inventory bucket, not this
// service, is authoritative for stock. Redis holds the admission mirror used by
// the later S2 reserve path.
type Service struct {
	pb.UnimplementedSeckillServiceServer
	db          *sql.DB
	inventory   pb.InventoryServiceClient
	orders      pb.OrderServiceClient
	payment     pb.PaymentServiceClient
	mq          *mq.Client
	cache       *redis.Client
	tokenSecret []byte
	log         *slog.Logger
	now         func() time.Time
}

func New(database *sql.DB, inventory pb.InventoryServiceClient, orders pb.OrderServiceClient, payment pb.PaymentServiceClient, client *mq.Client, cache *redis.Client, logger *slog.Logger, tokenSecret string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: database, inventory: inventory, orders: orders, payment: payment, mq: client, cache: cache,
		tokenSecret: []byte(tokenSecret), log: logger, now: time.Now}
}

var _ pb.SeckillServiceServer = (*Service)(nil)

func (s *Service) CreateActivity(ctx context.Context, req *pb.CreateSeckillActivityRequest) (*pb.SeckillActivity, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	a, err := s.createActivity(ctx, req)
	if err != nil {
		return nil, err
	}
	return a.toPB(), nil
}

func (s *Service) UpdateActivity(ctx context.Context, req *pb.UpdateSeckillActivityRequest) (*pb.SeckillActivity, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	if err := validateFields(req.GetCampaignId(), req.GetSkuId(), req.GetPriceCent(), req.GetStockTotal(),
		req.GetPerUserLimit(), req.GetStartAtUnix(), req.GetEndAtUnix()); err != nil {
		return nil, err
	}
	a, err := s.updateActivity(ctx, req)
	if err != nil {
		return nil, err
	}
	return a.toPB(), nil
}

func (s *Service) GetActivity(ctx context.Context, req *pb.GetSeckillActivityRequest) (*pb.SeckillActivity, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	a, err := s.getActivity(ctx, req.GetId(), false)
	if err != nil {
		return nil, err
	}
	return a.toPB(), nil
}

func (s *Service) ListActivities(ctx context.Context, req *pb.ListSeckillActivitiesRequest) (*pb.SeckillActivityList, error) {
	rows, total, err := s.listActivities(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.SeckillActivity, 0, len(rows))
	for _, a := range rows {
		out = append(out, a.toPB())
	}
	return &pb.SeckillActivityList{Activities: out, Total: total}, nil
}

// Publish allocates the DB stock bucket before making the activity visible.
// Replays are idempotent: inventory allocation and Redis warmup can be repeated.
func (s *Service) PublishActivity(ctx context.Context, req *pb.SeckillActivityCommand) (*pb.SeckillActivity, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	a, err := s.getActivity(ctx, req.GetId(), false)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if !a.endAt.After(now) {
		return nil, apperr.Conflict("seckill activity has already ended")
	}
	switch a.status {
	case statusActive, statusReady:
		// Already published; warmup is safe to repeat.
	case statusDraft:
		// Continue below.
	default:
		return nil, apperr.Conflict("only a draft activity can be published")
	}

	stock, err := s.inventory.AllocateSeckillStock(ctx, &pb.AllocateSeckillStockRequest{
		ActivityId: a.id, SkuId: a.skuID, WarehouseId: a.warehouseID, Quantity: a.stockTotal,
	})
	if err != nil {
		return nil, err
	}
	target := statusReady
	if !now.Before(a.startAt) {
		target = statusActive
	}
	if a.status == statusDraft {
		changed, err := s.setStatus(ctx, a.id, []string{statusDraft}, target)
		if err != nil {
			return nil, err
		}
		if !changed {
			return nil, apperr.Conflict("seckill activity changed while publishing")
		}
		a.status = target
	}
	if err := s.warmup(ctx, a, stock); err != nil {
		return nil, err
	}
	return a.toPB(), nil
}

// Pause releases only the unsold part of the inventory bucket. Held/sold units
// are not silently returned; that policy belongs to the order/refund workflow.
func (s *Service) PauseActivity(ctx context.Context, req *pb.SeckillActivityCommand) (*pb.SeckillActivity, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	a, err := s.getActivity(ctx, req.GetId(), false)
	if err != nil {
		return nil, err
	}
	if a.status == statusPaused {
		return a.toPB(), nil
	}
	if a.status != statusReady && a.status != statusActive {
		return nil, apperr.Conflict("only a ready or active activity can be paused")
	}
	if _, err := s.inventory.ReleaseSeckillStock(ctx, &pb.ReleaseSeckillStockRequest{ActivityId: a.id}); err != nil {
		return nil, err
	}
	changed, err := s.setStatus(ctx, a.id, []string{statusReady, statusActive}, statusPaused)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, apperr.Conflict("seckill activity changed while pausing")
	}
	s.clearWarmup(ctx, a.id)
	a.status = statusPaused
	return a.toPB(), nil
}

func (s *Service) WarmupActivity(ctx context.Context, req *pb.SeckillActivityCommand) (*pb.SeckillActivity, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	a, err := s.getActivity(ctx, req.GetId(), false)
	if err != nil {
		return nil, err
	}
	if a.status != statusReady && a.status != statusActive {
		return nil, apperr.Conflict("only a ready or active activity can be warmed up")
	}
	stock, err := s.inventory.GetSeckillStock(ctx, &pb.GetSeckillStockRequest{ActivityId: a.id})
	if err != nil {
		return nil, err
	}
	if err := s.warmup(ctx, a, stock); err != nil {
		return nil, err
	}
	return a.toPB(), nil
}

func (s *Service) GetActivityStock(ctx context.Context, req *pb.SeckillActivityCommand) (*pb.SeckillStock, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	return s.inventory.GetSeckillStock(ctx, &pb.GetSeckillStockRequest{ActivityId: req.GetId()})
}

func (s *Service) warmup(ctx context.Context, a *activity, stock *pb.SeckillStock) error {
	if s.cache == nil {
		return apperr.Unavailable("seckill redis is not configured", fmt.Errorf("redis client is nil"))
	}
	metaKey, stockKey := warmupKeys(a.id)
	ttl := a.endAt.Sub(s.now()) + warmupTTL
	if ttl <= 0 {
		ttl = warmupTTL
	}
	_, err := s.cache.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, metaKey, map[string]any{
			"activity_id": a.id, "campaign_id": a.campaignID, "sku_id": a.skuID,
			"warehouse_id": a.warehouseID, "price_cent": a.priceCent, "per_user_limit": a.perUserLimit,
			"start_at": a.startAt.Unix(), "end_at": a.endAt.Unix(), "status": a.status, "version": a.version,
		})
		pipe.Expire(ctx, metaKey, ttl)
		pipe.Set(ctx, stockKey, stock.GetAvailable(), ttl)
		return nil
	})
	if err != nil {
		s.log.Error("warm seckill activity", "activity_id", a.id, "error", err)
		return apperr.Unavailable("warm seckill stock: "+err.Error(), err)
	}
	return nil
}

func (s *Service) clearWarmup(ctx context.Context, activityID int64) {
	if s.cache == nil {
		return
	}
	metaKey, stockKey := warmupKeys(activityID)
	if err := s.cache.Del(ctx, metaKey, stockKey).Err(); err != nil {
		s.log.Warn("clear seckill warmup", "activity_id", activityID, "error", err)
	}
}

func warmupKeys(activityID int64) (string, string) {
	return fmt.Sprintf("seckill:{%d}:meta", activityID), fmt.Sprintf("seckill:{%d}:stock", activityID)
}

func validateCreate(req *pb.CreateSeckillActivityRequest) error {
	if req == nil {
		return apperr.Invalid("request is required")
	}
	return validateFields(req.GetCampaignId(), req.GetSkuId(), req.GetPriceCent(), req.GetStockTotal(),
		req.GetPerUserLimit(), req.GetStartAtUnix(), req.GetEndAtUnix())
}

func validateFields(campaignID, skuID, priceCent, stockTotal int64, perUserLimit int32, startUnix, endUnix int64) error {
	if campaignID <= 0 {
		return apperr.Invalid("campaign_id is required")
	}
	if skuID <= 0 {
		return apperr.Invalid("sku_id is required")
	}
	if priceCent <= 0 {
		return apperr.Invalid("price_cent must be positive")
	}
	if stockTotal <= 0 {
		return apperr.Invalid("stock_total must be positive")
	}
	if perUserLimit <= 0 {
		return apperr.Invalid("per_user_limit must be positive")
	}
	if endUnix <= startUnix {
		return apperr.Invalid("end_at must be after start_at")
	}
	return nil
}

func (s *Service) RunScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.activateDue(ctx)
			s.expireDue(ctx)
		}
	}
}

func (s *Service) activateDue(ctx context.Context) {
	rows, err := s.dueActivities(ctx)
	if err != nil {
		s.log.Warn("query due seckill activities", "error", err)
		return
	}
	for _, a := range rows {
		changed, err := s.setStatus(ctx, a.id, []string{statusReady}, statusActive)
		if err != nil || !changed {
			if err != nil {
				s.log.Warn("activate seckill activity", "activity_id", a.id, "error", err)
			}
			continue
		}
		a.status = statusActive
		stock, err := s.inventory.GetSeckillStock(ctx, &pb.GetSeckillStockRequest{ActivityId: a.id})
		if err != nil {
			s.log.Warn("load seckill stock on activation", "activity_id", a.id, "error", err)
			continue
		}
		if err := s.warmup(ctx, a, stock); err != nil {
			s.log.Warn("warm seckill activity on activation", "activity_id", a.id, "error", err)
		}
	}
}

func (s *Service) expireDue(ctx context.Context) {
	rows, err := s.expiredActivities(ctx)
	if err != nil {
		s.log.Warn("query expired seckill activities", "error", err)
		return
	}
	for _, a := range rows {
		if _, err := s.inventory.ReleaseSeckillStock(ctx, &pb.ReleaseSeckillStockRequest{ActivityId: a.id}); err != nil {
			s.log.Warn("release expired seckill stock", "activity_id", a.id, "error", err)
			continue
		}
		changed, err := s.setStatus(ctx, a.id, []string{statusReady, statusActive}, statusEnded)
		if err != nil || !changed {
			if err != nil {
				s.log.Warn("end seckill activity", "activity_id", a.id, "error", err)
			}
			continue
		}
		s.clearWarmup(ctx, a.id)
	}
}
