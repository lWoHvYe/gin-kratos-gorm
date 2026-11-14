package id

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func New(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s%d%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
func Order() string       { return New("ORD") }
func Reservation() string { return New("RSV") }
func Payment() string     { return New("PAY") }
func Shipment() string    { return New("SHP") }
func AfterSale() string   { return New("AS") }
func Event() string       { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

// OrderForSeckill derives the order number from the qualification token hash.
// The seckill worker may receive the same JetStream command more than once; a
// deterministic order number makes the second attempt hit the existing order
// (and the inventory (order_no, sku_id) reservation) instead of creating a
// second reservation against the same held unit.
func OrderForSeckill(tokenHash string) string {
	sum := sha256.Sum256([]byte("seckill-order:" + tokenHash))
	return "ORD" + hex.EncodeToString(sum[:12])
}
