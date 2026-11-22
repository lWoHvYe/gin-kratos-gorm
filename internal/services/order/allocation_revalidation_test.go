package order

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

type fakeAllocationInventory struct {
	pb.InventoryServiceClient
	reply *pb.AllocateOrderReply
	err   error
}

func (f *fakeAllocationInventory) AllocateOrder(context.Context, *pb.AllocateOrderRequest, ...grpc.CallOption) (*pb.AllocateOrderReply, error) {
	return f.reply, f.err
}

func allocationReplyFor(quote *pb.Quote) *pb.AllocateOrderReply {
	reply := &pb.AllocateOrderReply{}
	for _, line := range quote.GetLines() {
		reply.Allocations = append(reply.Allocations, &pb.AllocateOrderAllocation{
			SkuId: line.GetSkuId(), WarehouseId: line.GetWarehouseId(), Quantity: line.GetQuantity(),
			LineNo: line.GetLineNo(), SplitSeq: line.GetSplitSeq(),
		})
		reply.Availability = append(reply.Availability, &pb.AllocateOrderAvailability{
			SkuId: line.GetSkuId(), WarehouseId: line.GetWarehouseId(), Available: 2,
			Priority: int32(line.GetLineNo() - 1), Rank: int32(line.GetLineNo() - 1),
		})
	}
	return reply
}

// AC-09: a changed allocation is rejected before any order is created.
func TestAllocationRevalidationRejectsChangedPlan(t *testing.T) {
	quote := validSplitQuote()
	changed := allocationReplyFor(quote)
	changed.Allocations[0].Quantity++
	svc := &Service{inventory: &fakeAllocationInventory{reply: changed}, now: time.Now}
	_, err := svc.revalidateAllocation(context.Background(), quote)
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("changed plan err = %v", err)
	}
	if apperr.From(err).Details["stock_changed"] != true {
		t.Fatalf("details = %+v", apperr.From(err).Details)
	}
	missing := allocationReplyFor(quote)
	missing.Availability = nil
	svc.inventory = &fakeAllocationInventory{reply: missing}
	if _, err := svc.revalidateAllocation(context.Background(), quote); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("missing availability err = %v", err)
	}
}

func TestAllocationRevalidationAcceptsSameMultiset(t *testing.T) {
	quote := validSplitQuote()
	svc := &Service{inventory: &fakeAllocationInventory{reply: allocationReplyFor(quote)}, now: time.Now}
	reply, err := svc.revalidateAllocation(context.Background(), quote)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetAvailability()) == 0 {
		t.Fatal("availability was dropped")
	}
}

func TestAllocationDecisionRecordsAvailability(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	quote := validSplitQuote()
	allocation := allocationReplyFor(quote)
	if err := svc.recordAllocationDecision(context.Background(), "ORD-AUDIT", quote, allocation); err != nil {
		t.Fatal(err)
	}
	var availabilityJSON, rulesJSON []byte
	if err := db.QueryRow(`SELECT availability_json, rule_hits_json FROM order_allocation_decisions
		WHERE order_no = 'ORD-AUDIT'`).Scan(&availabilityJSON, &rulesJSON); err != nil {
		t.Fatal(err)
	}
	if string(availabilityJSON) == "{}" || len(availabilityJSON) == 0 {
		t.Fatalf("availability_json is empty: %s", availabilityJSON)
	}
	var payload struct {
		Source     string                   `json:"source"`
		Request    []map[string]any         `json:"request"`
		Warehouses []map[string]interface{} `json:"warehouses"`
	}
	if err := json.Unmarshal(availabilityJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Source != "ORDER_REVALIDATION" || len(payload.Request) != 2 || len(payload.Warehouses) != 2 {
		t.Fatalf("availability payload = %+v", payload)
	}
	for _, warehouse := range payload.Warehouses {
		for _, key := range []string{"sku_id", "warehouse_id", "available", "priority", "rank"} {
			if _, ok := warehouse[key]; !ok {
				t.Fatalf("warehouse snapshot missing %s: %+v", key, warehouse)
			}
		}
	}
	var rules []string
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		t.Fatal(err)
	}
	if len(rules) < 3 {
		t.Fatalf("rule hits = %v", rules)
	}
}
