package order

import "testing"

func TestRankStatus(t *testing.T) {
	ordered := []string{statusPendingPayment, statusPaid, statusAllocated, statusShipped, statusCompleted}
	for i := 1; i < len(ordered); i++ {
		if rankStatus(ordered[i-1]) >= rankStatus(ordered[i]) {
			t.Fatalf("rank order broken: %s >= %s", ordered[i-1], ordered[i])
		}
	}
	if rankStatus(statusCancelled) != -1 {
		t.Fatal("cancelled must not rank as a forward state")
	}
}
