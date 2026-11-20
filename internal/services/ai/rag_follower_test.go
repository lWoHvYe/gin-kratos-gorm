package ai

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestRAGVersionStale(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	fresh := ragVersion{CreatedAt: now.Add(-10 * time.Minute).Unix()}
	stale := ragVersion{CreatedAt: now.Add(-2 * time.Hour).Unix()}
	if ragVersionStale(fresh, now, time.Hour) {
		t.Fatal("fresh version marked stale")
	}
	if !ragVersionStale(stale, now, time.Hour) {
		t.Fatal("stale version marked fresh")
	}
	if !ragVersionStale(fresh, now, 0) {
		t.Fatal("maxAge=0 must always rebuild")
	}
	if !ragVersionStale(ragVersion{}, now, time.Hour) {
		t.Fatal("missing created_at must rebuild")
	}
	if ragCurrentKey(defaultRAGIndex) != "ai:rag:current" {
		t.Fatalf("default key changed: %q", ragCurrentKey(defaultRAGIndex))
	}
	if ragCurrentKey("ai_kb_test") != "ai_kb_test:current" {
		t.Fatalf("namespaced key = %q", ragCurrentKey("ai_kb_test"))
	}
}

func TestOpenRAGFollowerLoadsPublishedVersion(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the RAG follower test")
	}
	client := newTestRedisClient(addr)
	defer client.Close()
	ctx := context.Background()
	base := "ai_kb_follower_" + time.Now().Format("150405.000000000")
	key := ragCurrentKey(base)
	_ = client.Del(ctx, key).Err()
	defer client.Del(context.Background(), key)

	prefix := base + "_v1:"
	version := ragVersion{Index: base + "_current", Prefix: prefix, CreatedAt: time.Now().Unix()}
	if err := client.Set(ctx, key, mustJSON(version), 0).Err(); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRAGFollower(ctx, RAGConfig{Provider: "hash", Dim: 64, Index: base}, client, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	index, gotPrefix := r.ServingVersion()
	if index != version.Index || gotPrefix != prefix {
		t.Fatalf("serving version = %q/%q, want %q/%q", index, gotPrefix, version.Index, prefix)
	}

	// A follower must not invent a version when none is published.
	emptyBase := base + "_empty"
	if _, err := OpenRAGFollower(ctx, RAGConfig{Provider: "hash", Dim: 64, Index: emptyBase}, client, testLogger()); err == nil {
		t.Fatal("follower accepted a missing published version")
	}
}

func TestRunRAGFollowerFollowsPublishedPointer(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the RAG follower test")
	}
	client := newTestRedisClient(addr)
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := "ai_kb_follow_" + time.Now().Format("150405.000000000")
	key := ragCurrentKey(base)
	_ = client.Del(ctx, key).Err()
	defer client.Del(context.Background(), key)

	setVersion := func(prefix string) {
		t.Helper()
		version := ragVersion{Index: base + "_current", Prefix: prefix, CreatedAt: time.Now().Unix()}
		if err := client.Set(ctx, key, mustJSON(version), 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	setVersion(base + "_v1:")

	var mu sync.Mutex
	var seen []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunRAGFollower(ctx, RAGConfig{Provider: "hash", Dim: 64, Index: base}, client, testLogger(), 20*time.Millisecond, func(index RAGIndex) {
			r, ok := index.(*redisRAG)
			if !ok {
				return
			}
			mu.Lock()
			seen = append(seen, r.prefix)
			mu.Unlock()
		})
	}()
	waitForPrefix := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			mu.Lock()
			for _, got := range seen {
				if got == want {
					mu.Unlock()
					return
				}
			}
			mu.Unlock()
			if time.Now().After(deadline) {
				mu.Lock()
				got := append([]string(nil), seen...)
				mu.Unlock()
				t.Fatalf("follower never loaded %q; saw %v", want, got)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitForPrefix(base + "_v1:")
	setVersion(base + "_v2:")
	waitForPrefix(base + "_v2:")
	cancel()
	<-done
}

func TestRAGBuildDueFreshness(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the RAG build-due test")
	}
	client := newTestRedisClient(addr)
	defer client.Close()
	ctx := context.Background()
	base := "ai_kb_due_" + time.Now().Format("150405.000000000")
	key := ragCurrentKey(base)
	_ = client.Del(ctx, key).Err()
	defer client.Del(context.Background(), key)

	due, _, err := RAGBuildDue(ctx, client, base, time.Hour, false)
	if err != nil || !due {
		t.Fatalf("missing current: due=%v err=%v", due, err)
	}
	versioned := base + "_v1"
	if err := client.Do(ctx, "FT.CREATE", versioned, "ON", "HASH", "PREFIX", "1", versioned+":",
		"SCHEMA", "kind", "TAG").Err(); err != nil {
		t.Skipf("Redis Search unavailable: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_ = client.Do(bg, "FT.ALIASDEL", base+"_current").Err()
		_ = client.Do(bg, "FT.DROPINDEX", versioned, "DD").Err()
	})
	if err := client.Do(ctx, "FT.ALIASADD", base+"_current", versioned).Err(); err != nil {
		t.Fatal(err)
	}
	version := ragVersion{Index: base + "_current", Prefix: versioned + ":", CreatedAt: time.Now().Unix()}
	if err := client.Set(ctx, key, mustJSON(version), 0).Err(); err != nil {
		t.Fatal(err)
	}
	due, current, err := RAGBuildDue(ctx, client, base, time.Hour, false)
	if err != nil || due || current.Prefix != version.Prefix {
		t.Fatalf("fresh current: due=%v prefix=%q err=%v", due, current.Prefix, err)
	}
	if due, _, _ := RAGBuildDue(ctx, client, base, 0, false); !due {
		t.Fatal("maxAge=0 must force a rebuild")
	}
	// A fresh pointer whose index was dropped must self-heal with a rebuild.
	version.Index = base + "_missing"
	if err := client.Set(ctx, key, mustJSON(version), 0).Err(); err != nil {
		t.Fatal(err)
	}
	if due, _, _ := RAGBuildDue(ctx, client, base, time.Hour, false); !due {
		t.Fatal("pointer to a missing index must rebuild")
	}
	version.Index = base + "_current"
	if due, _, _ := RAGBuildDue(ctx, client, base, time.Hour, true); !due {
		t.Fatal("force must rebuild")
	}
	version.CreatedAt = time.Now().Add(-2 * time.Hour).Unix()
	if err := client.Set(ctx, key, mustJSON(version), 0).Err(); err != nil {
		t.Fatal(err)
	}
	if due, _, _ := RAGBuildDue(ctx, client, base, time.Hour, false); !due {
		t.Fatal("stale current must rebuild")
	}
}
