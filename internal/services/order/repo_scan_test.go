package order

import (
	"strings"
	"testing"
)

// TestOrderScanDestMatchesOrderColumns keeps orderColumns and orderScanDest in
// lockstep without needing a database. It fails when a column is added to the
// SELECT list but not to the shared destination list.
func TestOrderScanDestMatchesOrderColumns(t *testing.T) {
	columns := strings.Split(orderColumns, ",")
	dest := orderScanDest(&orderRecord{})
	if len(dest) != len(columns) {
		t.Fatalf("orderColumns declares %d columns but orderScanDest returns %d destinations; keep both in the same helper",
			len(columns), len(dest))
	}
}
