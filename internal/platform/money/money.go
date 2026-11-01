package money

import "fmt"

func Format(cents int64, currency string) string {
	if currency == "" {
		currency = "CNY"
	}
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%s%d.%02d", sign, currency, cents/100, cents%100)
}
func MulCents(price int64, qty int32) int64 { return price * int64(qty) }
