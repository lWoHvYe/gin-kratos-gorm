package aftersales

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

func restockAuthNo(afterSaleNo string, skuID int64) string {
	return fmt.Sprintf("RST:%s:%d", afterSaleNo, skuID)
}

// VerifyRestockAuthorization is called by Inventory for a cross-warehouse
// restock. Failure is fail-closed: Inventory must not accept the goods.
func (s *Service) VerifyRestockAuthorization(ctx context.Context, req *pb.VerifyRestockAuthorizationRequest) (*pb.VerifyRestockAuthorizationReply, error) {
	authNo := strings.TrimSpace(req.GetRestockAuthNo())
	if authNo == "" {
		return &pb.VerifyRestockAuthorizationReply{Valid: false, Reason: "missing_authorization"}, nil
	}
	var (
		afterSaleNo       string
		skuID, from, to   int64
		expiresAt, usedAt sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `SELECT after_sale_no, sku_id, from_warehouse_id, to_warehouse_id, expires_at, used_at
		FROM restock_exception_authorizations WHERE auth_no = ?`, authNo).
		Scan(&afterSaleNo, &skuID, &from, &to, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &pb.VerifyRestockAuthorizationReply{Valid: false, Reason: "not_found"}, nil
	}
	if err != nil {
		return nil, apperr.Internal("verify restock authorization", err)
	}
	if afterSaleNo != req.GetAfterSaleNo() || skuID != req.GetSkuId() ||
		req.GetFromWarehouseId() != from || req.GetToWarehouseId() != to {
		return &pb.VerifyRestockAuthorizationReply{Valid: false, Reason: "mismatch"}, nil
	}
	if !expiresAt.Valid || !expiresAt.Time.After(time.Now()) {
		return &pb.VerifyRestockAuthorizationReply{Valid: false, Reason: "expired"}, nil
	}
	if !usedAt.Valid {
		if _, err := s.db.ExecContext(ctx, `UPDATE restock_exception_authorizations SET used_at = NOW(3) WHERE auth_no = ?`,
			authNo); err != nil {
			return nil, apperr.Internal("mark restock authorization used", err)
		}
	}
	return &pb.VerifyRestockAuthorizationReply{Valid: true}, nil
}
