package outbox

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestChaosRelaySurvivesNatsOutage is driven by scripts/chaos-nats.sh, which
// stops and restarts the NATS container while this test relays events. The test
// inserts events, waits for the outage window, then keeps relaying until every
// event is published or the deadline expires.
func TestChaosRelaySurvivesNatsOutage(t *testing.T) {
	if os.Getenv("CHAOS_NATS") == "" {
		t.Skip("set CHAOS_NATS=1 (run scripts/chaos-nats.sh)")
	}
	db := integrationDB(t)
	client := natsClient(t)
	ctx := context.Background()

	const events = 20
	for i := 0; i < events; i++ {
		insertEvent(t, db, fmt.Sprintf("chaos-%d", i))
	}
	// The script stops NATS a few seconds after the test starts; waiting
	// here guarantees the relay meets the outage with unpublished rows.
	time.Sleep(8 * time.Second)

	deadline := time.Now().Add(90 * time.Second)
	var published int
	for time.Now().Before(deadline) {
		if _, err := relayOnce(ctx, db, client, testLogger()); err != nil {
			t.Logf("relay error during chaos: %v", err)
		}
		if err := db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE published_at IS NOT NULL").Scan(&published); err != nil {
			t.Fatal(err)
		}
		if published == events {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if published != events {
		t.Fatalf("recovered %d/%d events after the NATS outage", published, events)
	}
	// Dead-lettering must not have swallowed the events during the outage.
	var dead int
	if err := db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE dead_lettered_at IS NOT NULL").Scan(&dead); err != nil {
		t.Fatal(err)
	}
	if dead != 0 {
		t.Fatalf("%d events were dead-lettered during a transient outage", dead)
	}
}
