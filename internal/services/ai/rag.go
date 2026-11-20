package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"

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
	ReconcileProducts(ctx context.Context, catalog catalogClient) error
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
	r := &redisRAG{client: client, embedder: emb, base: index, log: logger}
	// Only one replica builds a new version. Others load the current alias;
	// this avoids every Pod embedding the catalog on startup.
	lockOwner := id.Event()
	lockKey := index + ":build:lock"
	const ragLockTTL = 3 * time.Minute
	locked, err := client.SetNX(ctx, lockKey, lockOwner, ragLockTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("rag build lock: %w", err)
	}
	if !locked {
		if current, ok := waitForCurrent(ctx, client, index, 60*time.Second); ok {
			r.index, r.prefix = current.Index, current.Prefix
			logger.Info("ai rag loaded current version", "index", current.Index, "prefix", current.Prefix)
			return r, nil
		}
		// The pointer may be missing while a valid alias still serves traffic.
		if current, ok := loadAliasVersion(ctx, client, index+"_current"); ok {
			r.index, r.prefix = current.Index, current.Prefix
			logger.Warn("ai rag loaded alias without current pointer", "index", current.Index, "prefix", current.Prefix)
			return r, nil
		}
		// Wait up to the remaining lease once, rather than failing while a slow
		// builder is still alive; renewal keeps an active builder's lease fresh.
		if ttl, err := client.PTTL(ctx, lockKey).Result(); err == nil && ttl > 0 {
			if current, ok := waitForCurrent(ctx, client, index, ttl+5*time.Second); ok {
				r.index, r.prefix = current.Index, current.Prefix
				logger.Info("ai rag loaded current version after lease wait", "index", current.Index, "prefix", current.Prefix)
				return r, nil
			}
		}
		// The lock holder died without publishing a version; take over after the
		// short lease expires.
		if locked, err = client.SetNX(ctx, lockKey, lockOwner, ragLockTTL).Result(); err != nil || !locked {
			return nil, fmt.Errorf("rag build lock is held and no current version is available")
		}
	}
	renewCtx, stopRenew := context.WithCancel(ctx)
	defer stopRenew()
	go renewRAGLock(renewCtx, client, lockKey, lockOwner)
	defer releaseRAGLock(client, lockKey, lockOwner)

	buildStart := time.Now()
	docs, err := collectKnowledge(ctx, catalog)
	if err != nil {
		return nil, err
	}
	if err := r.reindex(ctx, docs); err != nil {
		return nil, err
	}
	metrics.AIRAGDocuments.Set(float64(len(docs)))
	metrics.AIRAGBuildSeconds.Set(time.Since(buildStart).Seconds())
	// No TTL: this pointer outlives any single build and must remain readable by
	// non-leader replicas across restarts.
	if err := client.Set(ctx, "ai:rag:current", mustJSON(ragVersion{Index: r.index, Prefix: r.prefix, CreatedAt: time.Now().Unix()}), 0).Err(); err != nil {
		return nil, fmt.Errorf("publish rag current: %w", err)
	}
	// Catch changes that landed while the snapshot was being embedded.
	if catalog != nil {
		if err := r.ReconcileProducts(ctx, catalog); err != nil {
			logger.Warn("ai rag post-build reconcile failed", "error", err)
		} else {
			logger.Info("ai rag post-build reconcile complete")
		}
	}
	logger.Info("ai rag indexed", "index", index, "documents", len(docs), "embedding_provider", cfg.Provider, "dim", emb.Dim())
	return r, nil
}

func waitForCurrent(ctx context.Context, client redis.UniversalClient, base string, timeout time.Duration) (ragVersion, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if raw, err := client.Get(ctx, "ai:rag:current").Bytes(); err == nil {
			var current ragVersion
			if json.Unmarshal(raw, &current) == nil && current.Prefix != "" {
				return current, true
			}
		}
		select {
		case <-ctx.Done():
			return ragVersion{}, false
		case <-time.After(time.Second):
		}
	}
	return ragVersion{}, false
}

func loadAliasVersion(ctx context.Context, client redis.UniversalClient, alias string) (ragVersion, bool) {
	info, err := client.Do(ctx, "FT.INFO", alias).Slice()
	if err != nil {
		return ragVersion{}, false
	}
	for i := 0; i+1 < len(info); i += 2 {
		if fmt.Sprint(info[i]) == "index_name" {
			name := fmt.Sprint(info[i+1])
			if name != "" && name != alias {
				return ragVersion{Index: alias, Prefix: name + ":", CreatedAt: time.Now().Unix()}, true
			}
		}
	}
	return ragVersion{}, false
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func renewRAGLock(ctx context.Context, client redis.UniversalClient, key, owner string) {
	script := redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = script.Run(ctx, client, []string{key}, owner, int64(3*time.Minute/time.Millisecond)).Err()
		}
	}
}

var releaseRAGLock = func(client redis.UniversalClient, key, owner string) {
	script := redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
	_ = script.Run(context.Background(), client, []string{key}, owner).Err()
}

type ragVersion struct {
	Index     string `json:"index"`
	Prefix    string `json:"prefix"`
	CreatedAt int64  `json:"created_at"`
}

type redisRAG struct {
	client   redis.UniversalClient
	embedder embedder
	base     string
	index    string // current alias used for searches
	prefix   string // current versioned key prefix used for writes
	log      *slog.Logger
}

func collectKnowledge(ctx context.Context, catalog catalogClient) ([]ragDoc, error) {
	metrics.AIRAGTruncated.Set(0)
	var docs []ragDoc
	for _, p := range policies {
		docs = append(docs, ragDoc{ID: "policy:" + p.Title, Kind: "policy", Title: p.Title, Content: p.Content})
	}
	if catalog != nil {
		for page := int32(1); page <= 100; page++ {
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
			if page == 100 {
				metrics.AIRAGTruncated.Set(1)
			}
		}
	}
	return docs, nil
}

func (r *redisRAG) reindex(ctx context.Context, docs []ragDoc) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to index")
	}
	versioned := fmt.Sprintf("%s_v%d", r.base, time.Now().UnixNano())
	prefix := versioned + ":"
	create := []interface{}{
		"FT.CREATE", versioned, "ON", "HASH", "PREFIX", "1", prefix,
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
	// Register before writing docs so other replicas' consumers can dual-write
	// catalog events into this version while it is being built.
	encodedVersion, _ := json.Marshal(ragVersion{Index: versioned, Prefix: prefix, CreatedAt: time.Now().Unix()})
	if err := r.client.SAdd(ctx, "ai:rag:versions", encodedVersion).Err(); err != nil {
		return fmt.Errorf("register rag version: %w", err)
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Title + "\n" + d.Content
	}
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
		key := prefix + d.ID
		pipe.HSet(ctx, key,
			"kind", d.Kind,
			"title", d.Title,
			"content", d.Content,
			"terms", lexicalTerms(d.Title+"\n"+d.Content),
			"product_id", d.ProductID,
			"content_hash", contentHash(d.Title+"\n"+d.Content),
			"embedding", encodeVector(vectors[i]),
		)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store rag documents: %w", err)
	}

	alias := r.base + "_current"
	if err := r.client.Do(ctx, "FT.ALIASUPDATE", alias, versioned).Err(); err != nil {
		if addErr := r.client.Do(ctx, "FT.ALIASADD", alias, versioned).Err(); addErr != nil {
			return fmt.Errorf("switch rag alias: update=%v add=%v", err, addErr)
		}
	}
	r.index = alias
	r.prefix = prefix
	r.pruneVersions(ctx)
	return nil
}

// pruneVersions drops dual-write registrations older than seven days so a
// rolling deployment does not grow the fan-out forever.
func (r *redisRAG) pruneVersions(ctx context.Context) {
	values, err := r.client.SMembers(ctx, "ai:rag:versions").Result()
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	for _, raw := range values {
		var version ragVersion
		if json.Unmarshal([]byte(raw), &version) != nil || version.CreatedAt == 0 || version.CreatedAt < cutoff {
			_ = r.client.SRem(ctx, "ai:rag:versions", raw).Err()
		}
	}
	if count, err := r.client.SCard(ctx, "ai:rag:versions").Result(); err == nil {
		metrics.AIRAGVersions.Set(float64(count))
	}
}

// writeIndex returns the concrete index currently receiving writes; used by
// tests/ops that need to drop a versioned index rather than the alias.
func (r *redisRAG) writeIndex() string { return strings.TrimSuffix(r.prefix, ":") }

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
	cacheKey := r.queryCacheKey(query, limit)
	if cached, err := r.client.Get(ctx, cacheKey).Bytes(); err == nil {
		var docs []ragDoc
		if json.Unmarshal(cached, &docs) == nil {
			metrics.AIRAGSearch.WithLabelValues("cache").Inc()
			return docs, nil
		}
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
	if docs, ok := r.semanticLookup(ctx, vec[0], limit); ok {
		metrics.AIRAGSearch.WithLabelValues("semantic_cache").Inc()
		return docs, nil
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
		docs := rerankLexical(query, knn[:min(len(knn), limit)])
		r.cacheResults(ctx, cacheKey, docs)
		r.semanticStore(ctx, vec[0], docs)
		metrics.AIRAGSearch.WithLabelValues("knn_fallback").Inc()
		return docs, nil
	}
	docs := rerankLexical(query, fuseRRF(knn, lexical, limit))
	r.cacheResults(ctx, cacheKey, docs)
	r.semanticStore(ctx, vec[0], docs)
	metrics.AIRAGSearch.WithLabelValues("hybrid").Inc()
	return docs, nil
}

type semanticEntry struct {
	Vector []float32 `json:"v"`
	Docs   []ragDoc  `json:"d"`
}

func (r *redisRAG) semanticKey() string {
	return "ai:rag:sem:" + r.writeIndex()
}

func (r *redisRAG) semanticLookup(ctx context.Context, vec []float32, limit int) ([]ragDoc, bool) {
	values, err := r.client.LRange(ctx, r.semanticKey(), 0, 99).Result()
	if err != nil {
		return nil, false
	}
	best := 0.0
	var bestDocs []ragDoc
	for _, raw := range values {
		var entry semanticEntry
		if json.Unmarshal([]byte(raw), &entry) != nil || len(entry.Docs) < limit {
			continue
		}
		score := cosine(vec, entry.Vector)
		if score > best {
			best = score
			bestDocs = entry.Docs[:limit]
		}
	}
	if best >= 0.97 {
		return bestDocs, true
	}
	return nil, false
}

func (r *redisRAG) semanticStore(ctx context.Context, vec []float32, docs []ragDoc) {
	if len(docs) == 0 {
		return
	}
	entry := semanticEntry{Vector: vec, Docs: docs}
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	key := r.semanticKey()
	_ = r.client.LPush(ctx, key, payload).Err()
	_ = r.client.LTrim(ctx, key, 0, 99).Err()
	_ = r.client.Expire(ctx, key, 10*time.Minute).Err()
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

func (r *redisRAG) queryCacheKey(query string, limit int) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(query)) + "|" + fmt.Sprint(limit)))
	return "ai:rag:q:" + r.writeIndex() + ":" + hex.EncodeToString(sum[:8])
}

func (r *redisRAG) cacheResults(ctx context.Context, key string, docs []ragDoc) {
	if len(docs) == 0 {
		metrics.AIRAGEmpty.Inc()
	}
	payload, err := json.Marshal(docs)
	if err != nil {
		return
	}
	_ = r.client.Set(ctx, key, payload, 5*time.Minute).Err()
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
	hash := contentHash(doc.Title + "\n" + doc.Content)
	if existing, err := r.client.HGet(ctx, r.prefix+doc.ID, "content_hash").Result(); err == nil && existing == hash {
		return nil
	}
	vec, err := r.embedder.Embed(ctx, []string{doc.Title + "\n" + doc.Content})
	if err != nil || len(vec) != 1 {
		return fmt.Errorf("embed product %d: %w", p.GetId(), err)
	}
	versions := r.activeVersions(ctx)
	if len(versions) == 0 {
		versions = []ragVersion{{Index: r.writeIndex(), Prefix: r.prefix}}
	}
	pipe := r.client.Pipeline()
	for _, version := range versions {
		pipe.HSet(ctx, version.Prefix+doc.ID,
			"kind", doc.Kind, "title", doc.Title, "content", doc.Content,
			"terms", lexicalTerms(doc.Title+"\n"+doc.Content),
			"product_id", doc.ProductID, "content_hash", hash, "embedding", encodeVector(vec[0]))
	}
	if _, err = pipe.Exec(ctx); err != nil {
		return err
	}
	r.invalidateSearchCache(ctx)
	return nil
}

func (r *redisRAG) invalidateSearchCache(ctx context.Context) {
	_ = r.client.Del(ctx, r.semanticKey()).Err()
	pattern := "ai:rag:q:" + r.writeIndex() + ":*"
	iter := r.client.Scan(ctx, 0, pattern, 200).Iterator()
	for iter.Next(ctx) {
		_ = r.client.Del(ctx, iter.Val()).Err()
	}
}

func (r *redisRAG) activeVersions(ctx context.Context) []ragVersion {
	values, err := r.client.SMembers(ctx, "ai:rag:versions").Result()
	if err != nil {
		return nil
	}
	out := make([]ragVersion, 0, len(values))
	for _, raw := range values {
		var version ragVersion
		if json.Unmarshal([]byte(raw), &version) == nil && version.Prefix != "" {
			out = append(out, version)
		}
	}
	return out
}

// DeleteProduct removes a product from the index on unpublish/delete.
func (r *redisRAG) DeleteProduct(ctx context.Context, id int64) error {
	versions := r.activeVersions(ctx)
	if len(versions) == 0 {
		versions = []ragVersion{{Prefix: r.prefix}}
	}
	keys := make([]string, 0, len(versions))
	for _, version := range versions {
		keys = append(keys, version.Prefix+fmt.Sprintf("product:%d", id))
	}
	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return err
	}
	r.invalidateSearchCache(ctx)
	return nil
}

// ReconcileProducts catches events missed during an index build or consumed by
// an older replica. Unchanged products are skipped by content hash.
func (r *redisRAG) ReconcileProducts(ctx context.Context, catalog catalogClient) error {
	if catalog == nil {
		return nil
	}
	seen := map[int64]struct{}{}
	for page := int32(1); page <= 100; page++ {
		list, err := catalog.ListProducts(ctx, &pb.ListProductsRequest{Page: page, PageSize: 100})
		if err != nil {
			return fmt.Errorf("reconcile list products: %w", err)
		}
		for _, p := range list.GetProducts() {
			seen[p.GetId()] = struct{}{}
			if err := r.UpsertProduct(ctx, p); err != nil {
				return fmt.Errorf("reconcile product %d: %w", p.GetId(), err)
			}
		}
		if len(list.GetProducts()) < 100 {
			break
		}
	}
	iter := r.client.Scan(ctx, 0, r.prefix+"product:*", 200).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		idStr := strings.TrimPrefix(key, r.prefix+"product:")
		var id int64
		if _, err := fmt.Sscan(idStr, &id); err != nil {
			continue
		}
		if _, ok := seen[id]; !ok {
			if err := r.client.Del(ctx, key).Err(); err != nil {
				return err
			}
		}
	}
	return iter.Err()
}

func contentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
