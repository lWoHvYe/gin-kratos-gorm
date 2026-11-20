package ai

import (
	"context"
	"fmt"
	"sync"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// lazyRAG lets the AI service start before the embedding provider is reachable.
// The background builder swaps in a real index when it succeeds; tools fail
// closed until then and the assistant remains optional.
type lazyRAG struct {
	mu    sync.RWMutex
	inner RAGIndex
}

func NewLazyRAG() *lazyRAG { return &lazyRAG{} }

func (l *lazyRAG) Set(index RAGIndex) {
	l.mu.Lock()
	l.inner = index
	l.mu.Unlock()
}

func (l *lazyRAG) current() (RAGIndex, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.inner == nil {
		return nil, fmt.Errorf("rag index is not ready")
	}
	return l.inner, nil
}

func (l *lazyRAG) Search(ctx context.Context, query string, limit int) ([]ragDoc, error) {
	index, err := l.current()
	if err != nil {
		return nil, err
	}
	return index.Search(ctx, query, limit)
}

func (l *lazyRAG) UpsertProduct(ctx context.Context, p *pb.Product) error {
	index, err := l.current()
	if err != nil {
		return err
	}
	return index.UpsertProduct(ctx, p)
}

func (l *lazyRAG) DeleteProduct(ctx context.Context, id int64) error {
	index, err := l.current()
	if err != nil {
		return err
	}
	return index.DeleteProduct(ctx, id)
}

func (l *lazyRAG) ReconcileProducts(ctx context.Context, catalog catalogClient) error {
	index, err := l.current()
	if err != nil {
		return err
	}
	return index.ReconcileProducts(ctx, catalog)
}
