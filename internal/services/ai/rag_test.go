package ai

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHashEmbedderIsDeterministic(t *testing.T) {
	e := newHashEmbedder(64)
	a, _ := e.Embed(context.Background(), []string{"适合大学生的手机"})
	b, _ := e.Embed(context.Background(), []string{"适合大学生的手机"})
	c, _ := e.Embed(context.Background(), []string{"降噪耳机"})
	if len(a[0]) != 64 {
		t.Fatalf("dim = %d", len(a[0]))
	}
	if a[0][0] != b[0][0] || a[0][10] != b[0][10] {
		t.Fatal("embedding is not deterministic")
	}
	var norm float64
	for _, v := range a[0] {
		norm += float64(v * v)
	}
	if math.Abs(norm-1) > 0.001 {
		t.Fatalf("norm = %f", norm)
	}
	same := true
	for i := range a[0] {
		if a[0][i] != c[0][i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different texts produced the same vector")
	}
}

func TestOpenAIEmbedderContract(t *testing.T) {
	var auth, model string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		model = body.Model
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{0, 1, 0}},
				{"index": 0, "embedding": []float32{1, 0, 0}},
			},
		})
	}))
	defer srv.Close()

	e, err := newEmbedder(RAGConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "k", Model: "bge-m3", Dim: 3})
	if err != nil {
		t.Fatal(err)
	}
	vecs, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer k" || model != "bge-m3" {
		t.Fatalf("auth=%q model=%q", auth, model)
	}
	if len(vecs) != 2 || vecs[0][0] != 1 || vecs[1][1] != 1 {
		t.Fatalf("order not preserved: %+v", vecs)
	}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRAGRedisIndexAndSearch(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the Redis Search integration test")
	}
	client := newTestRedisClient(addr)
	ctx := context.Background()
	defer client.Close()

	index := "ai_kb_test_" + time.Now().Format("150405.000")
	rag, err := OpenRAG(ctx, RAGConfig{Provider: "hash", Dim: 64, Index: index}, &fakeCatalog{}, client, testLogger())
	if err != nil {
		t.Skipf("Redis Search unavailable: %v", err)
	}
	defer func() {
		_ = client.Do(context.Background(), "FT.ALIASDEL", index+"_current").Err()
		_ = client.Do(context.Background(), "FT.DROPINDEX", rag.writeIndex(), "DD").Err()
	}()

	docs, err := rag.Search(ctx, "键盘", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 {
		t.Fatal("no documents returned")
	}
	if docs[0].Title == "" {
		t.Fatalf("missing title: %+v", docs[0])
	}
}

func TestFuseRRFRanksAgreementFirst(t *testing.T) {
	knn := []ragDoc{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}
	lexical := []ragDoc{{ID: "b", Title: "B"}, {ID: "c", Title: "C"}}
	got := fuseRRF(knn, lexical, 2)
	if len(got) != 2 || got[0].ID != "b" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Score <= got[1].Score {
		t.Fatalf("scores not descending: %+v", got)
	}
}

func TestLexicalTermsSplitsCJK(t *testing.T) {
	terms := lexicalTerms("降噪耳机")
	for _, want := range []string{"降", "噪", "降噪", "耳机"} {
		if !strings.Contains(terms, want) {
			t.Fatalf("terms %q missing %q", terms, want)
		}
	}
}

func TestRerankLexicalBoostsExactCoverage(t *testing.T) {
	docs := []ragDoc{
		{ID: "a", Title: "无关内容", Content: "咖啡豆", Score: 0.03},
		{ID: "b", Title: "降噪耳机", Content: "降噪耳机", Score: 0.02},
	}
	got := rerankLexical("降噪耳机", docs)
	if got[0].ID != "b" {
		t.Fatalf("rerank did not boost exact coverage: %+v", got)
	}
}
