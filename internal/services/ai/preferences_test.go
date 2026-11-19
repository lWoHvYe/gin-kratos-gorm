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
