package ai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/redis/go-redis/v9"
)

// ragDoc is one retrievable knowledge chunk (product or policy).
type ragDoc struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	ProductID int64   `json:"product_id,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

type RAGSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]ragDoc, error)
}

// embedder turns text into a vector. Redis stores/searches the vectors; it does
// not generate embeddings, so a provider is always required.
type embedder interface {
	Dim() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// RAGConfig selects the embedding backend and the Redis index.
type RAGConfig struct {
	Index    string
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Dim      int
}

// OpenRAG builds the embedding provider, indexes the current catalog and the
// curated policy entries into Redis Search, and returns the searcher.
func OpenRAG(ctx context.Context, cfg RAGConfig, catalog catalogClient, client redis.UniversalClient, logger *slog.Logger) (*redisRAG, error) {
	if client == nil {
		return nil, fmt.Errorf("redis is required for rag")
	}
	emb, err := newEmbedder(cfg)
	if err != nil {
		return nil, err
	}
	index := strings.TrimSpace(cfg.Index)
	if index == "" {
		index = "ai_kb"
	}
	r := &redisRAG{client: client, embedder: emb, index: index, prefix: index + ":", log: logger}
	docs, err := collectKnowledge(ctx, catalog)
	if err != nil {
		return nil, err
	}
	if err := r.reindex(ctx, docs); err != nil {
		return nil, err
	}
	logger.Info("ai rag indexed", "index", index, "documents", len(docs), "embedding_provider", cfg.Provider, "dim", emb.Dim())
	return r, nil
}

type redisRAG struct {
	client   redis.UniversalClient
	embedder embedder
	index    string
	prefix   string
	log      *slog.Logger
}

func collectKnowledge(ctx context.Context, catalog catalogClient) ([]ragDoc, error) {
	var docs []ragDoc
	for _, p := range policies {
		docs = append(docs, ragDoc{ID: "policy:" + p.Title, Kind: "policy", Title: p.Title, Content: p.Content})
	}
	if catalog != nil {
		for page := int32(1); page <= 5; page++ {
			list, err := catalog.ListProducts(ctx, &pb.ListProductsRequest{Page: page, PageSize: 100})
			if err != nil {
				return nil, fmt.Errorf("list products for rag: %w", err)
			}
			for _, p := range list.GetProducts() {
				attrs := make([]string, 0, len(p.GetAttributes()))
				for _, a := range p.GetAttributes() {
					if a.GetName() != "" {
						attrs = append(attrs, a.GetName()+": "+a.GetValue())
					}
				}
				content := strings.TrimSpace(strings.Join([]string{
					p.GetName(), p.GetSubtitle(), p.GetDescription(),
					"价格：" + fmt.Sprintf("%.2f 元", float64(p.GetMinPriceCent())/100),
					"规格：" + strings.Join(attrs, "；"),
				}, "\n"))
				docs = append(docs, ragDoc{
					ID: fmt.Sprintf("product:%d", p.GetId()), Kind: "product",
					Title: p.GetName(), Content: content, ProductID: p.GetId(),
				})
			}
			if len(list.GetProducts()) < 100 {
				break
			}
		}
	}
	return docs, nil
}

func (r *redisRAG) reindex(ctx context.Context, docs []ragDoc) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to index")
	}
	// Recreate the index on startup: it is small and this avoids stale rows
	// after catalog changes until an event-driven indexer exists.
	_ = r.client.Do(ctx, "FT.DROPINDEX", r.index, "DD").Err()
	create := []interface{}{
		"FT.CREATE", r.index, "ON", "HASH", "PREFIX", "1", r.prefix,
		"SCHEMA",
		"kind", "TAG",
		"title", "TEXT",
		"content", "TEXT",
		"product_id", "NUMERIC",
		"embedding", "VECTOR", "HNSW", "6", "TYPE", "FLOAT32", "DIM", fmt.Sprint(r.embedder.Dim()),
		"DISTANCE_METRIC", "COSINE",
	}
	if err := r.client.Do(ctx, create...).Err(); err != nil {
		return fmt.Errorf("create rag index: %w", err)
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Title + "\n" + d.Content
	}
	// Embedding APIs cap batch sizes (Qwen's endpoint is small), so embed in
	// fixed chunks and concatenate in document order.
	const batchSize = 10
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := r.embedder.Embed(ctx, texts[start:end])
		if err != nil {
			return fmt.Errorf("embed documents: %w", err)
		}
		if len(batch) != end-start {
			return fmt.Errorf("embedding batch count %d != %d", len(batch), end-start)
		}
		vectors = append(vectors, batch...)
	}
	if len(vectors) != len(docs) {
		return fmt.Errorf("embedding count %d != documents %d", len(vectors), len(docs))
	}
	pipe := r.client.Pipeline()
	for i, d := range docs {
		key := r.prefix + d.ID
		pipe.HSet(ctx, key,
			"kind", d.Kind,
			"title", d.Title,
			"content", d.Content,
			"product_id", d.ProductID,
			"embedding", encodeVector(vectors[i]),
		)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store rag documents: %w", err)
	}
	return nil
}

func (r *redisRAG) Search(ctx context.Context, query string, limit int) ([]ragDoc, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 8 {
		limit = 8
	}
	vec, err := r.embedder.Embed(ctx, []string{query})
	if err != nil || len(vec) != 1 {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	args := []interface{}{
		"FT.SEARCH", r.index,
		fmt.Sprintf("*=>[KNN %d @embedding $vec AS score]", limit),
		"PARAMS", "2", "vec", encodeVector(vec[0]),
		"SORTBY", "score", "DIALECT", "2",
		"RETURN", "4", "kind", "title", "content", "product_id",
	}
	raw, err := r.client.Do(ctx, args...).Result()
	if err != nil {
		return nil, fmt.Errorf("rag search: %w", err)
	}
	return parseRAGReply(raw, r.prefix), nil
}

// parseRAGReply accepts both RESP2 (flat array) and RESP3 (map) FT.SEARCH
// replies. go-redis negotiates RESP3 with Redis 8, while older servers/tests
// return the flat array.
func parseRAGReply(raw interface{}, prefix string) []ragDoc {
	var out []ragDoc
	add := func(key string, fields map[interface{}]interface{}) {
		doc := ragDoc{ID: strings.TrimPrefix(key, prefix)}
		for name, value := range fields {
			name := fmt.Sprint(name)
			value := fmt.Sprint(value)
			switch name {
			case "kind":
				doc.Kind = value
			case "title":
				doc.Title = value
			case "content":
				doc.Content = value
			case "product_id":
				fmt.Sscan(value, &doc.ProductID)
			}
		}
		out = append(out, doc)
	}

	switch v := raw.(type) {
	case []interface{}:
		for i := 1; i+1 < len(v); i += 2 {
			key, _ := v[i].(string)
			fields, ok := v[i+1].([]interface{})
			if !ok {
				continue
			}
			flat := map[interface{}]interface{}{}
			for j := 0; j+1 < len(fields); j += 2 {
				flat[fields[j]] = fields[j+1]
			}
			add(key, flat)
		}
	case map[interface{}]interface{}:
		results, _ := v["results"].([]interface{})
		for _, item := range results {
			entry, ok := item.(map[interface{}]interface{})
			if !ok {
				continue
			}
			key := fmt.Sprint(entry["id"])
			fields, _ := entry["extra_attributes"].(map[interface{}]interface{})
			add(key, fields)
		}
	case map[string]interface{}:
		results, _ := v["results"].([]interface{})
		for _, item := range results {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			fields := map[interface{}]interface{}{}
			if attrs, ok := entry["extra_attributes"].(map[string]interface{}); ok {
				for k, val := range attrs {
					fields[k] = val
				}
			}
			add(fmt.Sprint(entry["id"]), fields)
		}
	}
	return out
}

// ---------------------------------------------------------------- embedders

func newEmbedder(cfg RAGConfig) (embedder, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "hash":
		return newHashEmbedder(cfg.Dim), nil
	case "openai":
		if strings.TrimSpace(cfg.Model) == "" {
			return nil, fmt.Errorf("embedding model is required")
		}
		return &openAIEmbedder{
			baseURL: strings.TrimRight(cfg.BaseURL, "/"),
			apiKey:  cfg.APIKey,
			model:   cfg.Model,
			dim:     cfg.Dim,
			client:  &http.Client{Timeout: 30 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", cfg.Provider)
	}
}

// hashEmbedder is a deterministic lexical fallback (token hashing). It is not
// semantic: it keeps local/dev/CI RAG functional until a real embedding model
// is configured, and must not be presented as semantic search.
type hashEmbedder struct{ dim int }

func newHashEmbedder(dim int) *hashEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return &hashEmbedder{dim: dim}
}

func (h *hashEmbedder) Dim() int { return h.dim }

func (h *hashEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = h.embed(text)
	}
	return out, nil
}

func (h *hashEmbedder) embed(text string) []float32 {
	vec := make([]float32, h.dim)
	for _, tok := range tokenize(text) {
		sum := fnv.New32a()
		_, _ = sum.Write([]byte(tok))
		idx := int(sum.Sum32() % uint32(h.dim))
		vec[idx] += 1
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	if norm > 0 {
		inv := float32(1 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var word []rune
	var cjk []rune
	flushWord := func() {
		if len(word) > 0 {
			tokens = append(tokens, string(word))
			word = word[:0]
		}
	}
	flushCJK := func() {
		for i, r := range cjk {
			tokens = append(tokens, string(r))
			if i+1 < len(cjk) {
				tokens = append(tokens, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			word = append(word, r)
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return tokens
}

type openAIEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	dim     int
	client  *http.Client
}

func (o *openAIEmbedder) Dim() int { return o.dim }

func (o *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, _ := json.Marshal(map[string]any{"model": o.model, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding endpoint returned %s", resp.Status)
	}
	var reply struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return nil, err
	}
	if len(reply.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count %d != texts %d", len(reply.Data), len(texts))
	}
	sort.Slice(reply.Data, func(i, j int) bool { return reply.Data[i].Index < reply.Data[j].Index })
	out := make([][]float32, len(reply.Data))
	for i, item := range reply.Data {
		if o.dim > 0 && len(item.Embedding) != o.dim {
			return nil, fmt.Errorf("embedding dim %d != configured %d", len(item.Embedding), o.dim)
		}
		out[i] = item.Embedding
	}
	return out, nil
}

func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
