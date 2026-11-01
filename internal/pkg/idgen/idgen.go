// Package idgen generates sortable, collision-resistant identifiers.
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"sync/atomic"
	"time"
)

// OrderSN generates order numbers of the form:
//
//	yyyyMMddHHmmss + 3-digit node + 6-digit sequence
//
// The sequence is process-local and monotonically increasing. The node id is
// derived from the hostname, which keeps ids unique across replicas without a
// central sequence service.
type OrderSN struct {
	node uint64
	seq  atomic.Uint64
	now  func() time.Time
}

func NewOrderSN(node string) *OrderSN {
	if node == "" {
		node, _ = os.Hostname()
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(node))
	return &OrderSN{node: uint64(h.Sum32()), now: time.Now}
}

// WithClock allows deterministic tests.
func (g *OrderSN) WithClock(now func() time.Time) *OrderSN {
	g.now = now
	return g
}

func (g *OrderSN) Next() string {
	seq := g.seq.Add(1) % 1_000_000
	return fmt.Sprintf("%s%03d%06d", g.now().Format("20060102150405"), g.node%1000, seq)
}

// EventID returns a random 128-bit hex id for outbox events.
func EventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
