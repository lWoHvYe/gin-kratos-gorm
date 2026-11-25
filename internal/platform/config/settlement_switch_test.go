package config

import (
	"os"
	"strings"
	"testing"
)

// Test matrix for the R0.4b settlement switch configuration (ADR-0007 Rev 2.3
// §3.2/§3.4, Addendum-r0.4 §2.1). The DB-side approval row checks live in
// internal/services/fulfillment.

func unsetSettlementEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"FULFILLMENT_SETTLE_CONSUMER_MODE",
		"SETTLE_RELEASE_APPROVAL_ID",
		"SETTLE_RELEASE_RANGE",
		"FULFILLMENT_CONSUME_ORDER_SETTLED",
	} {
		// t.Setenv registers the original state for cleanup; unsetting after
		// that makes LookupEnv see a genuinely absent variable, which is what
		// the ADR's "unset" phrases mean (an explicit empty value is invalid).
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

func TestSettlementSwitchConfigMatrix(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		approval string
		rangeVal string
		wantMode string
		wantErr  string
	}{
		{name: "default is paid", wantMode: FulfillmentSettleConsumerPaid},
		{name: "explicit paid", mode: "paid", wantMode: FulfillmentSettleConsumerPaid},
		{
			name: "paid with approval id is N5",
			mode: "paid", approval: "apr-1", wantErr: ReasonN5ApprovalIDModeMismatch,
		},
		{
			name:     "approval id alone is N5",
			approval: "apr-1", wantErr: ReasonN5ApprovalIDModeMismatch,
		},
		{
			name: "shadow needs release range",
			mode: "shadow", wantErr: ReasonReleaseRangeMissing,
		},
		{
			name: "shadow with approval id is N5 even with range",
			mode: "shadow", approval: "apr-1", rangeVal: "prod", wantErr: ReasonN5ApprovalIDModeMismatch,
		},
		{
			name: "valid shadow",
			mode: "shadow", rangeVal: "prod", wantMode: FulfillmentSettleConsumerShadow,
		},
		{
			name: "settled needs approval id",
			mode: "settled", rangeVal: "prod", wantErr: "SETTLE_RELEASE_APPROVAL_ID",
		},
		{
			name: "settled needs release range",
			mode: "settled", approval: "apr-1", wantErr: ReasonReleaseRangeMissing,
		},
		{
			name: "valid settled",
			mode: "settled", approval: "apr-1", rangeVal: "prod", wantMode: FulfillmentSettleConsumerSettled,
		},
		{
			name: "empty mode is invalid",
			mode: " ", wantErr: ReasonModeUnknown,
		},
		{
			name: "unknown mode is invalid",
			mode: "settled-prod", wantErr: ReasonModeUnknown,
		},
		{
			name: "uppercase mode is invalid",
			mode: "PAID", wantErr: ReasonModeUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unsetSettlementEnv(t)
			if tc.mode != "" {
				t.Setenv("FULFILLMENT_SETTLE_CONSUMER_MODE", tc.mode)
			}
			if tc.approval != "" {
				t.Setenv("SETTLE_RELEASE_APPROVAL_ID", tc.approval)
			}
			if tc.rangeVal != "" {
				t.Setenv("SETTLE_RELEASE_RANGE", tc.rangeVal)
			}
			c := &Config{}
			err := applySettlementConsumerEnv(c)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.FulfillmentSettleConsumerMode != tc.wantMode {
				t.Fatalf("mode = %q, want %q", c.FulfillmentSettleConsumerMode, tc.wantMode)
			}
		})
	}
}

func TestSettlementSwitchRangeIsTrimmed(t *testing.T) {
	unsetSettlementEnv(t)
	t.Setenv("FULFILLMENT_SETTLE_CONSUMER_MODE", "shadow")
	t.Setenv("SETTLE_RELEASE_RANGE", "  prod  ")
	c := &Config{}
	if err := applySettlementConsumerEnv(c); err != nil {
		t.Fatal(err)
	}
	if c.SettleReleaseRange != "prod" {
		t.Fatalf("range = %q, want prod", c.SettleReleaseRange)
	}
}

func TestSettlementSwitchLegacyBooleanMatrix(t *testing.T) {
	t.Run("legacy true maps to gated settled", func(t *testing.T) {
		unsetSettlementEnv(t)
		t.Setenv("FULFILLMENT_CONSUME_ORDER_SETTLED", "true")
		c := &Config{}
		if err := applySettlementConsumerEnv(c); err == nil || !strings.Contains(err.Error(), "SETTLE_RELEASE_APPROVAL_ID") {
			t.Fatalf("err = %v, want missing approval id", err)
		}
	})
	t.Run("legacy true with approval id still needs range", func(t *testing.T) {
		unsetSettlementEnv(t)
		t.Setenv("FULFILLMENT_CONSUME_ORDER_SETTLED", "true")
		t.Setenv("SETTLE_RELEASE_APPROVAL_ID", "apr-1")
		c := &Config{}
		if err := applySettlementConsumerEnv(c); err == nil || !strings.Contains(err.Error(), ReasonReleaseRangeMissing) {
			t.Fatalf("err = %v, want missing range token", err)
		}
	})
	t.Run("legacy false is paid and rejects approval id", func(t *testing.T) {
		unsetSettlementEnv(t)
		t.Setenv("FULFILLMENT_CONSUME_ORDER_SETTLED", "false")
		t.Setenv("SETTLE_RELEASE_APPROVAL_ID", "apr-1")
		c := &Config{}
		if err := applySettlementConsumerEnv(c); err == nil || !strings.Contains(err.Error(), ReasonN5ApprovalIDModeMismatch) {
			t.Fatalf("err = %v, want the N5 token", err)
		}
	})
	t.Run("new and legacy conflict fails", func(t *testing.T) {
		unsetSettlementEnv(t)
		t.Setenv("FULFILLMENT_SETTLE_CONSUMER_MODE", "shadow")
		t.Setenv("SETTLE_RELEASE_RANGE", "prod")
		t.Setenv("FULFILLMENT_CONSUME_ORDER_SETTLED", "true")
		c := &Config{}
		if err := applySettlementConsumerEnv(c); err == nil || !strings.Contains(err.Error(), "conflicts") {
			t.Fatalf("err = %v, want conflict", err)
		}
	})
}

// TestLoadSettlementDefaultPaid exercises the real Load path so a stray
// approval id or range cannot silently enable a gated mode.
func TestLoadSettlementDefaultPaid(t *testing.T) {
	clearConfigEnv(t)
	unsetSettlementEnv(t)
	c, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if c.FulfillmentSettleConsumerMode != FulfillmentSettleConsumerPaid {
		t.Fatalf("mode = %q", c.FulfillmentSettleConsumerMode)
	}
	if c.SettleReleaseApprovalID != "" || c.SettleReleaseRange != "" {
		t.Fatalf("approval/range = %q/%q", c.SettleReleaseApprovalID, c.SettleReleaseRange)
	}
}
