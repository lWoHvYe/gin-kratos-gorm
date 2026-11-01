package id

import (
	"crypto/rand"
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
