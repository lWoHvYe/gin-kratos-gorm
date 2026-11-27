package order

import (
	"context"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestAC59ShortageResolutionBeforeReportRejected(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	seedTccOrder(t, db, "ORD-AC59-SHORT", 10000, 10000, 0, 0, "PAY-AC59-SHORT")
	_, err := svc.ChooseShortageResolution(context.Background(), &pb.ChooseShortageResolutionRequest{
		OrderNo: "ORD-AC59-SHORT", GroupNo: "G1", UserId: 1, Resolution: "REFUND",
	})
	if err == nil || !strings.Contains(err.Error(), "EVENT_OUT_OF_ORDER") {
		t.Fatalf("shortage resolution error = %v", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shortage_cases WHERE order_no = 'ORD-AC59-SHORT'`); got != 0 {
		t.Fatalf("out-of-order resolution created %d cases", got)
	}
}
