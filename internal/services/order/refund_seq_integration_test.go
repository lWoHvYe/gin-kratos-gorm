package order

import (
	"context"
	"sync"
	"testing"
)

func TestLegacyRefundSeqConcurrentUnique(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	ctx := context.Background()
	const callers = 20
	seqs := make([]int64, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			seq, err := svc.nextLegacyRefundSeq(ctx, "ORD-SEQ")
			if err != nil {
				errs[index] = err
				return
			}
			seqs[index] = seq
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	seen := make(map[int64]bool, callers)
	for _, seq := range seqs {
		if seq < 1 || seq > callers {
			t.Fatalf("sequence %d out of range 1..%d", seq, callers)
		}
		if seen[seq] {
			t.Fatalf("duplicate sequence %d", seq)
		}
		seen[seq] = true
	}
	var next int64
	if err := db.QueryRow(`SELECT next_seq FROM order_refund_sequences WHERE order_no = 'ORD-SEQ'`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if next != callers {
		t.Fatalf("next_seq = %d, want %d", next, callers)
	}
	if got := legacyRefundID("ORD-SEQ", 7); got != "legacy:ORD-SEQ:7" {
		t.Fatalf("refund id = %q", got)
	}
}

// TestLegacyRefundSeqRollbackDoesNotReuse is the ADR-0010 contract: the
// sequence is committed independently, so a later refund transaction rollback
// must not make the next allocation reuse the same number.
func TestLegacyRefundSeqRollbackDoesNotReuse(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	ctx := context.Background()
	caller, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.nextLegacyRefundSeq(ctx, "ORD-SEQ-ROLLBACK")
	if err != nil {
		_ = caller.Rollback()
		t.Fatal(err)
	}
	if err := caller.Rollback(); err != nil {
		t.Fatal(err)
	}
	second, err := svc.nextLegacyRefundSeq(ctx, "ORD-SEQ-ROLLBACK")
	if err != nil {
		t.Fatal(err)
	}
	if second != first+1 {
		t.Fatalf("sequence reused after rollback: first=%d second=%d", first, second)
	}
}

func TestLegacyRefundSeqRejectsEmptyOrder(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	if _, err := svc.nextLegacyRefundSeq(context.Background(), "  "); err == nil {
		t.Fatal("empty order_no was accepted")
	}
}

func TestLegacyRefundSeqRequiresDB(t *testing.T) {
	if _, err := (&Service{}).nextLegacyRefundSeq(context.Background(), "ORD-SEQ"); err == nil {
		t.Fatal("nil db was accepted")
	}
}
