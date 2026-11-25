package order

import (
	"errors"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// T1.4b1-R unit evidence for the pure classification/mapping helpers that the
// MySQL integration tests cannot exhaust (malformed Payment replies, every
// deterministic error code).

func TestValidateTccRefundBatchReply(t *testing.T) {
	request := &pb.RefundBatchRequest{
		PaymentNo: "PAY-1", BatchId: "tcc:PAY-1:1", ExpectedTotalCent: 10000,
		Items: []*pb.RefundBatchItem{
			{RefundId: "tcc:PAY-1:G1", AmountCent: 6000},
			{RefundId: "tcc:PAY-1:G2", AmountCent: 4000},
		},
	}
	valid := func() *pb.RefundBatchReply {
		return &pb.RefundBatchReply{
			BatchId: "tcc:PAY-1:1", Status: refundBatchStatusRefunded, RefundedCent: 10000,
			ItemResults: []*pb.RefundBatchItemResult{
				{RefundId: "tcc:PAY-1:G1", RefundNo: "RFD-1", Status: refundBatchStatusRefunded, AmountCent: 6000},
				{RefundId: "tcc:PAY-1:G2", RefundNo: "RFD-2", Status: refundBatchStatusAlreadyRefunded, AmountCent: 4000},
			},
		}
	}
	if err := validateTccRefundBatchReply(valid(), request); err != nil {
		t.Fatalf("valid reply rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*pb.RefundBatchReply)
	}{
		{"nil reply", nil},
		{"wrong batch", func(r *pb.RefundBatchReply) { r.BatchId = "tcc:PAY-1:2" }},
		{"wrong status", func(r *pb.RefundBatchReply) { r.Status = "PENDING" }},
		{"wrong total", func(r *pb.RefundBatchReply) { r.RefundedCent = 9000 }},
		{"missing item", func(r *pb.RefundBatchReply) { r.ItemResults = r.ItemResults[:1] }},
		{"extra item", func(r *pb.RefundBatchReply) {
			r.ItemResults = append(r.ItemResults, &pb.RefundBatchItemResult{
				RefundId: "tcc:PAY-1:G3", RefundNo: "RFD-3", Status: refundBatchStatusRefunded, AmountCent: 1,
			})
		}},
		{"wrong amount", func(r *pb.RefundBatchReply) { r.ItemResults[0].AmountCent = 5999 }},
		{"bad item status", func(r *pb.RefundBatchReply) { r.ItemResults[0].Status = "FAILED" }},
		{"missing refund_no", func(r *pb.RefundBatchReply) { r.ItemResults[1].RefundNo = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate == nil {
				if err := validateTccRefundBatchReply(nil, request); err == nil {
					t.Fatal("nil reply accepted")
				}
				return
			}
			reply := valid()
			tc.mutate(reply)
			if err := validateTccRefundBatchReply(reply, request); err == nil {
				t.Fatalf("invalid reply accepted: %+v", reply)
			}
		})
	}
}

func TestClassifyTccRefundBatchError(t *testing.T) {
	grpcConflict := func(reason string) error {
		st := status.New(codes.AlreadyExists, reason)
		with, err := st.WithDetails(&errdetails.ErrorInfo{
			Domain: "mall", Reason: "CONFLICT", Metadata: map[string]string{"reason": reason},
		})
		if err != nil {
			t.Fatal(err)
		}
		return with.Err()
	}
	cases := []struct {
		name          string
		err           error
		wantReason    string
		deterministic bool
	}{
		{"batch unavailable", apperr.Unavailable("BATCH_UNAVAILABLE: db", nil).
			WithDetails(map[string]any{"reason": refundBatchReasonUnavailable}),
			refundBatchReasonUnavailable, false},
		{"insufficient remaining", apperr.Failed("INSUFFICIENT_REMAINING: changed").
			WithDetails(map[string]any{"reason": "INSUFFICIENT_REMAINING"}),
			"INSUFFICIENT_REMAINING", true},
		{"idempotency conflict grpc", grpcConflict("IDEMPOTENCY_CONFLICT"), "IDEMPOTENCY_CONFLICT", true},
		{"raw transport error", errors.New("connection reset"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, deterministic := classifyTccRefundBatchError(tc.err)
			if reason != tc.wantReason || deterministic != tc.deterministic {
				t.Fatalf("classify = %q/%t, want %q/%t", reason, deterministic, tc.wantReason, tc.deterministic)
			}
		})
	}
}

func TestClassifyTccRefund(t *testing.T) {
	app := &tccApplicationRef{RefundID: "tcc:PAY-1:G1", GroupNo: "G1", AmountCent: 6000,
		State: refundApplicationProcessing}
	candidate := func(commitmentState, appState string) *tccPaymentCandidate {
		return &tccPaymentCandidate{
			Application: &tccApplicationRef{RefundID: app.RefundID, GroupNo: app.GroupNo,
				AmountCent: app.AmountCent, State: appState},
			Commitment: &tccPaymentCommitment{RefundID: app.RefundID, GroupNo: app.GroupNo,
				AmountCent: app.AmountCent, State: commitmentState},
		}
	}
	success := &pb.Refund{RefundId: app.RefundID, AmountCent: app.AmountCent, Status: tccRefundStatusSucceeded}
	pending := &pb.Refund{RefundId: app.RefundID, AmountCent: app.AmountCent, Status: tccRefundStatusProcessing}
	cases := []struct {
		name       string
		candidate  *tccPaymentCandidate
		fact       *pb.Refund
		wantClass  tccRefundClass
		wantReason string
	}{
		{"success fact with lagging ledger", candidate(commitmentReserved, refundApplicationProcessing),
			success, tccRefundDone, ""},
		{"success fact already settled", candidate(commitmentSettled, refundApplicationDone),
			success, tccRefundDone, ""},
		{"pending fact", candidate(commitmentReserved, refundApplicationProcessing),
			pending, tccRefundInFlight, ""},
		{"pending but settled", candidate(commitmentSettled, refundApplicationDone),
			pending, tccRefundUnknown, "REFUND_IN_FLIGHT_BUT_COMMITMENT_SETTLED"},
		{"amount mismatch", candidate(commitmentReserved, refundApplicationProcessing),
			&pb.Refund{RefundId: app.RefundID, AmountCent: 1, Status: tccRefundStatusSucceeded},
			tccRefundUnknown, "REFUND_AMOUNT_MISMATCH"},
		{"unknown status", candidate(commitmentReserved, refundApplicationProcessing),
			&pb.Refund{RefundId: app.RefundID, AmountCent: app.AmountCent, Status: "FAILED"},
			tccRefundUnknown, "REFUND_STATUS_UNRECOGNIZED"},
		{"settled without fact", candidate(commitmentSettled, refundApplicationDone),
			nil, tccRefundDone, ""},
		{"application done without fact", candidate(commitmentReserved, refundApplicationDone),
			nil, tccRefundUnknown, "APPLICATION_DONE_WITHOUT_PAYMENT_FACT"},
		{"reserved outstanding", candidate(commitmentReserved, refundApplicationProcessing),
			nil, tccRefundOutstanding, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, reason := classifyTccRefund(tc.candidate, tc.fact)
			if class != tc.wantClass || reason != tc.wantReason {
				t.Fatalf("classify = %v/%q, want %v/%q", class, reason, tc.wantClass, tc.wantReason)
			}
		})
	}
}
