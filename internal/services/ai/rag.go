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

// RAGIndex extends the searcher with incremental document updates.
type RAGIndex interface {
	RAGSearcher
	UpsertProduct(ctx context.Context, p *pb.Product) error
	DeleteProduct(ctx context.Context, id int64) error
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
				docs = append(docs, productDoc(p))
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
		"terms", "TEXT",
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
			"terms", lexicalTerms(d.Title+"\n"+d.Content),
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
	fetch := limit * 4
	if fetch < 20 {
		fetch = 20
	}
	if fetch > 40 {
		fetch = 40
	}
	vec, err := r.embedder.Embed(ctx, []string{query})
	if err != nil || len(vec) != 1 {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	knnArgs := []interface{}{
		"FT.SEARCH", r.index,
		fmt.Sprintf("*=>[KNN %d @embedding $vec AS score]", fetch),
		"PARAMS", "2", "vec", encodeVector(vec[0]),
		"SORTBY", "score", "DIALECT", "2",
		"RETURN", "4", "kind", "title", "content", "product_id",
	}
	raw, err := r.client.Do(ctx, knnArgs...).Result()
	if err != nil {
		return nil, fmt.Errorf("rag knn search: %w", err)
	}
	knn := parseRAGReply(raw, r.prefix)

	// Lexical retrieval uses a pre-tokenized field: Redis' default tokenizer
	// does not split CJK text, so querying raw title/content would miss Chinese.
	lexical, lexErr := r.lexicalSearch(ctx, query, fetch)
	if lexErr != nil {
		if r.log != nil {
			r.log.Warn("rag lexical search failed; using knn only", "error", lexErr)
		}
		return rerankLexical(query, knn[:min(len(knn), limit)]), nil
	}
	return rerankLexical(query, fuseRRF(knn, lexical, limit)), nil
}

// rerankLexical is a deterministic rerank: RRF rank is the base score, with a
// small bonus for documents that actually contain the query tokens. It avoids
// a second paid model while making exact-term matches win close ties.
func rerankLexical(query string, docs []ragDoc) []ragDoc {
	queryTokens := uniqueTokens(tokenize(query))
	if len(queryTokens) == 0 || len(docs) < 2 {
		return docs
	}
	wanted := make(map[string]struct{}, len(queryTokens))
	for _, tok := range queryTokens {
		wanted[tok] = struct{}{}
	}
	for i := range docs {
		docTokens := uniqueTokens(tokenize(docs[i].Title + "\n" + docs[i].Content))
		matched := 0
		for _, tok := range docTokens {
			if _, ok := wanted[tok]; ok {
				matched++
			}
		}
		docs[i].Score += 0.15 * float64(matched) / float64(len(queryTokens))
	}
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].Score > docs[j].Score })
	return docs
}

func (r *redisRAG) lexicalSearch(ctx context.Context, query string, limit int) ([]ragDoc, error) {
	tokens := uniqueTokens(tokenize(query))
	if len(tokens) == 0 {
		return nil, nil
	}
	escaped := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		escaped = append(escaped, escapeQueryToken(tok))
	}
	args := []interface{}{
		"FT.SEARCH", r.index,
		"@terms:(" + strings.Join(escaped, "|") + ")",
		"SCORER", "BM25",
		"LIMIT", "0", fmt.Sprint(limit),
		"RETURN", "4", "kind", "title", "content", "product_id",
	}
	raw, err := r.client.Do(ctx, args...).Result()
	if err != nil {
		return nil, err
	}
	return parseRAGReply(raw, r.prefix), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fuseRRF merges two ranked lists. k=60 is the standard RRF constant.
func fuseRRF(knn, lexical []ragDoc, limit int) []ragDoc {
	const k = 60.0
	scores := map[string]float64{}
	byID := map[string]ragDoc{}
	add := func(docs []ragDoc) {
		for rank, doc := range docs {
			if doc.ID == "" {
				continue
			}
			scores[doc.ID] += 1 / (k + float64(rank+1))
			if _, ok := byID[doc.ID]; !ok {
				byID[doc.ID] = doc
			}
		}
	}
	add(knn)
	add(lexical)
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] == scores[ids[j]] {
			return ids[i] < ids[j]
		}
		return scores[ids[i]] > scores[ids[j]]
	})
	out := make([]ragDoc, 0, min(limit, len(ids)))
	for _, id := range ids {
		doc := byID[id]
		doc.Score = scores[id]
		out = append(out, doc)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func uniqueTokens(tokens []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return out
}

func escapeQueryToken(tok string) string {
	var b strings.Builder
	for _, r := range tok {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func lexicalTerms(text string) string {
	return strings.Join(uniqueTokens(tokenize(text)), " ")
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

func productDoc(p *pb.Product) ragDoc {
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
	return ragDoc{
		ID: fmt.Sprintf("product:%d", p.GetId()), Kind: "product",
		Title: p.GetName(), Content: content, ProductID: p.GetId(),
	}
}

// UpsertProduct indexes one product without rebuilding the whole index.
func (r *redisRAG) UpsertProduct(ctx context.Context, p *pb.Product) error {
	doc := productDoc(p)
	vec, err := r.embedder.Embed(ctx, []string{doc.Title + "\n" + doc.Content})
	if err != nil || len(vec) != 1 {
		return fmt.Errorf("embed product %d: %w", p.GetId(), err)
	}
	return r.client.HSet(ctx, r.prefix+doc.ID,
		"kind", doc.Kind, "title", doc.Title, "content", doc.Content,
		"terms", lexicalTerms(doc.Title+"\n"+doc.Content),
		"product_id", doc.ProductID, "embedding", encodeVector(vec[0])).Err()
}

// DeleteProduct removes a product from the index on unpublish/delete.
func (r *redisRAG) DeleteProduct(ctx context.Context, id int64) error {
	return r.client.Del(ctx, r.prefix+fmt.Sprintf("product:%d", id)).Err()
}
