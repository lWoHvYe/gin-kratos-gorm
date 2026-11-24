package aftersales

import "testing"

func TestRefundAdmissionSourceUsesSystemShortage(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{sourceSystem, "system_shortage"},
		{"USER", "aftersale"},
		{"", "aftersale"},
	}
	for _, tc := range cases {
		row := &afterSaleRow{source: tc.source}
		if got := refundAdmissionSource(row); got != tc.want {
			t.Fatalf("source %q -> %q, want %q", tc.source, got, tc.want)
		}
	}
}
