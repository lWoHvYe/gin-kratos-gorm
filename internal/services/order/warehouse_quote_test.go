package order

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestValidateSingleWarehouseQuote(t *testing.T) {
	if err := validateSingleWarehouseQuote(&pb.Quote{}); err != nil {
		t.Fatalf("legacy quote rejected: %v", err)
	}
	if err := validateSingleWarehouseQuote(&pb.Quote{
		Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1}},
		Lines:  []*pb.QuoteLine{{WarehouseId: 1, GroupNo: "G1"}},
	}); err != nil {
		t.Fatalf("single group rejected: %v", err)
	}
	for _, quote := range []*pb.Quote{
		{Groups: []*pb.QuoteGroup{{GroupNo: "G1"}, {GroupNo: "G2"}}},
		{Lines: []*pb.QuoteLine{{WarehouseId: 2}}},
		{Lines: []*pb.QuoteLine{{WarehouseId: 1, GroupNo: "G2"}}},
	} {
		if err := validateSingleWarehouseQuote(quote); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("split quote accepted: %+v err=%v", quote, err)
		}
	}
}
