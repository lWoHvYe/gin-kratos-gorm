// Package refundpolicy holds the closed responsibility/reason enums shared by
// AfterSales (the producer) and Order (the validator). Keeping one source of
// truth prevents the two services from drifting on free-text responsibility.
package refundpolicy

import "strings"

const (
	ResponsibilityMerchant     = "MERCHANT_WAREHOUSE"
	ResponsibilityCarrier      = "CARRIER"
	ResponsibilityCustomer     = "CUSTOMER"
	ResponsibilityPlatform     = "PLATFORM"
	ResponsibilityUndetermined = "UNDETERMINED"
)

var reasonCodes = map[string]map[string]struct{}{
	ResponsibilityMerchant: {
		"OUT_OF_STOCK": {}, "WRONG_ITEM": {}, "MISSING_ITEM": {}, "DAMAGED_PACKAGING": {}, "LATE_DISPATCH": {},
	},
	ResponsibilityCarrier: {
		"LOST_IN_TRANSIT": {}, "DAMAGED_IN_TRANSIT": {}, "WRONG_DELIVERY": {},
	},
	ResponsibilityCustomer: {
		"NO_REASON_RETURN": {}, "REFUSED": {}, "ADDRESS_ERROR": {},
	},
	ResponsibilityPlatform: {
		"SYSTEM_DEFECT": {}, "PRICING_ERROR": {},
	},
	ResponsibilityUndetermined: {"PENDING_REVIEW": {}},
}

// Normalize upper-cases and trims an enum input. Callers pass the result to
// ValidResponsibility/ValidReason.
func Normalize(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// ValidResponsibility reports whether party is one of the closed enum values.
func ValidResponsibility(party string) bool {
	switch party {
	case ResponsibilityMerchant, ResponsibilityCarrier, ResponsibilityCustomer,
		ResponsibilityPlatform, ResponsibilityUndetermined:
		return true
	default:
		return false
	}
}

// ValidReason reports whether code is valid for the given party. The pair is
// closed; Order never infers a reason from free text.
func ValidReason(party, code string) bool {
	codes, ok := reasonCodes[party]
	if !ok {
		return false
	}
	_, ok = codes[code]
	return ok
}
