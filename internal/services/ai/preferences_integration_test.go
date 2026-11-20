package ai

import (
	"context"
	"os"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/redis/go-redis/v9"
)

func TestPreferencesAreUserScoped(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR")
	}
	cache := redis.NewClient(&redis.Options{Addr: addr})
	defer cache.Close()
	svc := &Service{cache: cache}
	ctx := context.Background()
	if _, err := svc.SetPreferences(ctx, &pb.SetAIPreferencesRequest{
		UserId: 1, Preferences: &pb.AIPreferences{PreferredBrands: []string{"云舟"}},
	}); err != nil {
		t.Fatal(err)
	}
	one, err := svc.GetPreferences(ctx, &pb.GetAIPreferencesRequest{UserId: 1})
	if err != nil || len(one.GetPreferredBrands()) != 1 {
		t.Fatalf("user1 = %+v err=%v", one, err)
	}
	two, err := svc.GetPreferences(ctx, &pb.GetAIPreferencesRequest{UserId: 2})
	if err != nil || len(two.GetPreferredBrands()) != 0 {
		t.Fatalf("user2 = %+v err=%v", two, err)
	}
	_ = cache.Del(ctx, preferenceKey(1)).Err()
}
