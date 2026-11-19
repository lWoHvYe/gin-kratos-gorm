package catalog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type productCursor struct {
	Sort  string `json:"s"`
	Price int64  `json:"p,omitempty"`
	ID    int64  `json:"i"`
}

func encodeProductCursor(sort string, price, id int64) string {
	raw, _ := json.Marshal(productCursor{Sort: sort, Price: price, ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeProductCursor(raw string) (*productCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c productCursor
	if err := json.Unmarshal(data, &c); err != nil || c.ID <= 0 {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

func normalizeSort(sort string) string {
	switch sort {
	case "price_asc", "price_desc":
		return sort
	default:
		return "newest"
	}
}
