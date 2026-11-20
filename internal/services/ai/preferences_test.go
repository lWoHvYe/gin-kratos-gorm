package ai

import (
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestPreferencePromptIsExplicitAndScoped(t *testing.T) {
	got := preferencePrompt(&pb.AIPreferences{
		PreferredBrands: []string{"云舟"}, BudgetMinCent: 100000, BudgetMaxCent: 300000,
	})
	if !strings.Contains(got, "云舟") || !strings.Contains(got, "100000-300000") || !strings.Contains(got, "跨用户") {
		t.Fatalf("prompt = %q", got)
	}
	if preferencePrompt(&pb.AIPreferences{}) != "" {
		t.Fatal("empty preferences must not inject a prompt")
	}
}

func TestPreferenceAttributionOmitsEmptyAndPromptText(t *testing.T) {
	attribution, ok := preferenceAttributionOf(&pb.AIPreferences{
		PreferredBrands: []string{"云舟"}, PreferredCategories: []string{"手机"},
		BudgetMinCent: 100000, BudgetMaxCent: 300000, Sizes: []string{"M"},
	})
	if !ok {
		t.Fatal("non-empty profile was not attributed")
	}
	raw := string(attribution.JSON())
	for _, want := range []string{`"brands":["云舟"]`, `"categories":["手机"]`, `"budget_min_cent":100000`, `"budget_max_cent":300000`, `"sizes":["M"]`, `"source":"explicit_profile"`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("attribution %s missing %s", raw, want)
		}
	}
	if strings.Contains(raw, "偏好") || strings.Contains(raw, "以下是用户") {
		t.Fatalf("attribution leaked prompt text: %s", raw)
	}
	if _, ok := preferenceAttributionOf(&pb.AIPreferences{}); ok {
		t.Fatal("empty profile must not be attributed")
	}
	if _, ok := preferenceAttributionOf(nil); ok {
		t.Fatal("nil profile must not be attributed")
	}
}
