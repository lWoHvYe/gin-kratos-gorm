package admin

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestValidateShipmentOrderUsesServerActions(t *testing.T) {
	if err := validateShipmentOrder(&pb.Order{AvailableActions: []string{"apply_after_sale", "ship"}}); err != nil {
		t.Fatalf("ship action rejected: %v", err)
	}
	for _, order := range []*pb.Order{
		{Status: "PAID", RefundState: "REFUNDING", AvailableActions: []string{"apply_after_sale"}},
		{Status: "SHIPPED", RefundState: "FULL", AvailableActions: []string{"receive"}},
		{},
	} {
		if err := validateShipmentOrder(order); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("order %+v err = %v", order, err)
		}
	}
}
