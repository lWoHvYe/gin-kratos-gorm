package aftersales

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestBuildRefundAdmissionRequest(t *testing.T) {
	lines := []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}}
	req, err := buildRefundAdmissionRequest("AS-1", "ORD-1", "G1", "aftersale", 1200,
		"MERCHANT_WAREHOUSE", "OUT_OF_STOCK", lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.GetLines()) != 1 || req.GetFreightRefundableCent() != 200 ||
		req.GetResponsibility() != "MERCHANT_WAREHOUSE" || req.GetReasonCode() != "OUT_OF_STOCK" {
		t.Fatalf("request = %+v", req)
	}
}

func TestBuildRefundAdmissionRequestLegacyAmountOnly(t *testing.T) {
	req, err := buildRefundAdmissionRequest("AS-2", "ORD-2", "G1", "aftersale", 5000, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.GetLines()) != 0 || req.GetAmountCent() != 5000 || req.GetFreightRefundableCent() != 0 {
		t.Fatalf("legacy request = %+v", req)
	}
}

func TestBuildRefundAdmissionRequestRejectsOverAmount(t *testing.T) {
	if _, err := buildRefundAdmissionRequest("AS-3", "ORD-3", "G1", "aftersale", 500,
		"MERCHANT_WAREHOUSE", "OUT_OF_STOCK", []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}}); err == nil {
		t.Fatal("lines over amount were accepted")
	}
}

func TestBuildRefundAdmissionRequestHoldsUndeterminedFreight(t *testing.T) {
	if _, err := buildRefundAdmissionRequest("AS-4", "ORD-4", "G1", "aftersale", 1200,
		"UNDETERMINED", "PENDING_REVIEW", []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}}); err == nil {
		t.Fatal("UNDETERMINED freight was released")
	}
}

func TestBuildRefundAdmissionRequestNormalizesResponsibility(t *testing.T) {
	// Lower-case input is normalized before the UNDETERMINED freight guard.
	if _, err := buildRefundAdmissionRequest("AS-5", "ORD-5", "G1", "aftersale", 1200,
		" undetermined ", "pending_review", []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}}); err == nil {
		t.Fatal("normalized UNDETERMINED freight was released")
	}
	req, err := buildRefundAdmissionRequest("AS-6", "ORD-6", "G1", "aftersale", 1200,
		"merchant_warehouse", "out_of_stock", []*pb.RefundLine{{LineNo: 1, Quantity: 1, GoodsCent: 1000}})
	if err != nil {
		t.Fatal(err)
	}
	if req.GetResponsibility() != "MERCHANT_WAREHOUSE" || req.GetReasonCode() != "OUT_OF_STOCK" {
		t.Fatalf("normalized request = %+v", req)
	}
}
