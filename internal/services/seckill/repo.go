package seckill

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	statusDraft   = "DRAFT"
	statusReady   = "READY"
	statusActive  = "ACTIVE"
	statusPaused  = "PAUSED"
	statusEnded   = "ENDED"
	statusSoldOut = "SOLD_OUT"
)

type activity struct {
	id           int64
	campaignID   int64
	skuID        int64
	warehouseID  int64
	name         string
	priceCent    int64
	stockTotal   int64
	perUserLimit int32
	startAt      time.Time
	endAt        time.Time
	status       string
	version      int64
	createdAt    time.Time
	updatedAt    time.Time
}

func (a *activity) toPB() *pb.SeckillActivity {
	return &pb.SeckillActivity{
		Id: a.id, CampaignId: a.campaignID, SkuId: a.skuID, WarehouseId: a.warehouseID,
		Name: a.name, PriceCent: a.priceCent, StockTotal: a.stockTotal,
		PerUserLimit: a.perUserLimit, StartAtUnix: a.startAt.Unix(), EndAtUnix: a.endAt.Unix(),
		Status: a.status, Version: a.version, CreatedAtUnix: a.createdAt.Unix(), UpdatedAtUnix: a.updatedAt.Unix(),
	}
}

const activityColumns = `id, campaign_id, sku_id, warehouse_id, name, price_cent, stock_total,
	per_user_limit, start_at, end_at, status, version, created_at, updated_at`

func scanActivity(row interface{ Scan(...any) error }) (*activity, error) {
	var a activity
	if err := row.Scan(&a.id, &a.campaignID, &a.skuID, &a.warehouseID, &a.name, &a.priceCent,
		&a.stockTotal, &a.perUserLimit, &a.startAt, &a.endAt, &a.status, &a.version, &a.createdAt, &a.updatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) createActivity(ctx context.Context, req *pb.CreateSeckillActivityRequest) (*activity, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = fmt.Sprintf("seckill-%d", req.GetSkuId())
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO seckill_activities
		(campaign_id, sku_id, warehouse_id, name, price_cent, stock_total, per_user_limit, start_at, end_at, status, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		req.GetCampaignId(), req.GetSkuId(), warehouseID(req.GetWarehouseId()), name, req.GetPriceCent(),
		req.GetStockTotal(), req.GetPerUserLimit(), time.Unix(req.GetStartAtUnix(), 0), time.Unix(req.GetEndAtUnix(), 0), statusDraft)
	if err != nil {
		if isDuplicate(err) {
			return nil, apperr.Conflict("seckill activity already exists for this campaign and sku")
		}
		return nil, apperr.Internal("create seckill activity", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, apperr.Internal("seckill activity id", err)
	}
	return s.getActivity(ctx, id, false)
}

func (s *Service) updateActivity(ctx context.Context, req *pb.UpdateSeckillActivityRequest) (*activity, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = fmt.Sprintf("seckill-%d", req.GetSkuId())
	}
	res, err := s.db.ExecContext(ctx, `UPDATE seckill_activities
		SET campaign_id=?, sku_id=?, warehouse_id=?, name=?, price_cent=?, stock_total=?, per_user_limit=?,
		    start_at=?, end_at=?, version=version+1
		WHERE id=? AND version=? AND status='DRAFT'`,
		req.GetCampaignId(), req.GetSkuId(), warehouseID(req.GetWarehouseId()), name, req.GetPriceCent(),
		req.GetStockTotal(), req.GetPerUserLimit(), time.Unix(req.GetStartAtUnix(), 0), time.Unix(req.GetEndAtUnix(), 0),
		req.GetId(), req.GetVersion())
	if err != nil {
		return nil, apperr.Internal("update seckill activity", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, apperr.Conflict("seckill activity was modified or is no longer editable")
	}
	return s.getActivity(ctx, req.GetId(), false)
}

func (s *Service) getActivity(ctx context.Context, id int64, forUpdate bool) (*activity, error) {
	query := `SELECT ` + activityColumns + ` FROM seckill_activities WHERE id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	a, err := scanActivity(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("seckill activity not found")
	}
	if err != nil {
		return nil, apperr.Internal("query seckill activity", err)
	}
	return a, nil
}

func (s *Service) listActivities(ctx context.Context, req *pb.ListSeckillActivitiesRequest) ([]*activity, int64, error) {
	page, size := normalizePage(req.GetPage(), req.GetPageSize())
	where := "1=1"
	args := []any{}
	if status := strings.ToUpper(strings.TrimSpace(req.GetStatus())); status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM seckill_activities WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal("count seckill activities", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, `SELECT `+activityColumns+` FROM seckill_activities WHERE `+where+
		` ORDER BY start_at DESC, id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, apperr.Internal("list seckill activities", err)
	}
	defer rows.Close()
	var out []*activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, 0, apperr.Internal("scan seckill activity", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Service) setStatus(ctx context.Context, id int64, from []string, to string) (bool, error) {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(from)), ",")
	args := []any{to, id}
	for _, v := range from {
		args = append(args, v)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE seckill_activities SET status=?, version=version+1
		WHERE id=? AND status IN (`+placeholders+`)`, args...)
	if err != nil {
		return false, apperr.Internal("update seckill activity status", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Service) dueActivities(ctx context.Context) ([]*activity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+activityColumns+` FROM seckill_activities
		WHERE status='READY' AND start_at <= NOW(3) AND end_at > NOW(3) ORDER BY id LIMIT 50`)
	if err != nil {
		return nil, apperr.Internal("query due seckill activities", err)
	}
	defer rows.Close()
	var out []*activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, apperr.Internal("scan due seckill activity", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) expiredActivities(ctx context.Context) ([]*activity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+activityColumns+` FROM seckill_activities
		WHERE status IN ('READY','ACTIVE') AND end_at <= NOW(3) ORDER BY id LIMIT 50`)
	if err != nil {
		return nil, apperr.Internal("query expired seckill activities", err)
	}
	defer rows.Close()
	var out []*activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, apperr.Internal("scan expired seckill activity", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func normalizePage(page, size int32) (int32, int32) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func warehouseID(id int64) int64 {
	if id <= 0 {
		return 1
	}
	return id
}

func isDuplicate(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "1062")
}

func (s *Service) listPublicActivities(ctx context.Context, req *pb.ListSeckillActivitiesRequest) ([]*activity, int64, error) {
	page, size := normalizePage(req.GetPage(), req.GetPageSize())
	where := `status IN ('READY','ACTIVE') AND end_at > NOW(3)`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM seckill_activities WHERE `+where).Scan(&total); err != nil {
		return nil, 0, apperr.Internal("count public seckill activities", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+activityColumns+` FROM seckill_activities WHERE `+where+
		` ORDER BY start_at ASC, id ASC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		return nil, 0, apperr.Internal("list public seckill activities", err)
	}
	defer rows.Close()
	var out []*activity
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, 0, apperr.Internal("scan public seckill activity", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
