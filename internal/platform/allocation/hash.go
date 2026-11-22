// Package allocation provides the canonical serialization shared by Pricing and
// Order. A quote's allocation_hash is computed at preview time and recomputed at
// order creation; any drift rejects the order instead of silently re-allocating.
package allocation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Line struct {
	LineNo       int32  `json:"line_no"`
	SkuID        int64  `json:"sku_id"`
	WarehouseID  int64  `json:"warehouse_id"`
	GroupNo      string `json:"group_no"`
	Quantity     int32  `json:"quantity"`
	PriceCent    int64  `json:"price_cent"`
	DiscountCent int64  `json:"discount_cent"`
	LinePaidCent int64  `json:"line_paid_cent"`
	UnitPaidCent int64  `json:"unit_paid_cent"`
}

type Group struct {
	GroupNo          string `json:"group_no"`
	WarehouseID      int64  `json:"warehouse_id"`
	GoodsCent        int64  `json:"goods_cent"`
	DiscountCent     int64  `json:"discount_cent"`
	FreightShareCent int64  `json:"freight_share_cent"`
}

type payload struct {
	V      int     `json:"v"`
	Lines  []Line  `json:"lines"`
	Groups []Group `json:"groups"`
}

// Hash returns the SHA-256 of the canonical line/group snapshot. The caller must
// pass lines already ordered by line_no and groups ordered by group_no.
func Hash(lines []Line, groups []Group) string {
	encoded, err := json.Marshal(payload{V: 1, Lines: lines, Groups: groups})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
