package refundpolicy

import "testing"

func TestClosedResponsibilityEnums(t *testing.T) {
	if !ValidResponsibility(ResponsibilityMerchant) || !ValidResponsibility(ResponsibilityUndetermined) {
		t.Fatal("known responsibility rejected")
	}
	if ValidResponsibility("MERCHANT") || ValidResponsibility("") {
		t.Fatal("unknown responsibility accepted")
	}
	if !ValidReason(ResponsibilityMerchant, "OUT_OF_STOCK") {
		t.Fatal("valid pair rejected")
	}
	if ValidReason(ResponsibilityMerchant, "LOST_IN_TRANSIT") {
		t.Fatal("cross-party reason accepted")
	}
	if !ValidReason(ResponsibilityUndetermined, "PENDING_REVIEW") {
		t.Fatal("undetermined pending pair rejected")
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("  merchant_warehouse "); got != ResponsibilityMerchant {
		t.Fatalf("normalize = %q", got)
	}
}
