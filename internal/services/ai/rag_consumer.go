package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const catalogChangedSubject = "shop.catalog.product.changed.v1"

type ragIndexer interface {
	UpsertProduct(ctx context.Context, p *pb.Product) error
	DeleteProduct(ctx context.Context, id int64) error
}

type catalogChangedEvent struct {
	SpuID  int64  `json:"spu_id"`
	Status string `json:"status"`
}

// RunCatalogIndexConsumer keeps the Redis vector index in sync with catalog
// changes. It runs only when RAG is enabled and NATS is reachable; a failure
// leaves the startup snapshot in place and is retried by the durable consumer.
func RunCatalogIndexConsumer(ctx context.Context, client *mq.Client, catalog catalogClient, indexer ragIndexer, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: catalogChangedSubject,
		Durable: "ai-catalog-index",
	}, func(hctx context.Context, msg *nats.Msg) error {
		ev, err := decodeCatalogEvent(msg.Data)
		if err != nil {
			return consumer.Permanent(err)
		}
		return applyCatalogChange(hctx, catalog, indexer, ev)
	})
	if err != nil && ctx.Err() == nil {
		logger.Error("ai catalog index consumer stopped", "error", err)
	}
}

func applyCatalogChange(ctx context.Context, catalog catalogClient, indexer ragIndexer, ev catalogChangedEvent) error {
	if ev.SpuID <= 0 {
		return consumer.Permanent(fmt.Errorf("catalog change has no spu_id"))
	}
	if !strings.EqualFold(ev.Status, "ON_SALE") {
		if err := indexer.DeleteProduct(ctx, ev.SpuID); err != nil {
			metrics.AIRAGUpdates.WithLabelValues("error").Inc()
			return err
		}
		metrics.AIRAGUpdates.WithLabelValues("deleted").Inc()
		return nil
	}
	p, err := catalog.GetProduct(ctx, &pb.GetProductRequest{Id: ev.SpuID})
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) || status.Code(err) == codes.NotFound {
			if derr := indexer.DeleteProduct(ctx, ev.SpuID); derr != nil {
				metrics.AIRAGUpdates.WithLabelValues("error").Inc()
				return derr
			}
			metrics.AIRAGUpdates.WithLabelValues("deleted").Inc()
			return nil
		}
		metrics.AIRAGUpdates.WithLabelValues("error").Inc()
		return err
	}
	if !strings.EqualFold(p.GetStatus(), "ON_SALE") {
		if derr := indexer.DeleteProduct(ctx, ev.SpuID); derr != nil {
			metrics.AIRAGUpdates.WithLabelValues("error").Inc()
			return derr
		}
		metrics.AIRAGUpdates.WithLabelValues("deleted").Inc()
		return nil
	}
	if err := indexer.UpsertProduct(ctx, p); err != nil {
		metrics.AIRAGUpdates.WithLabelValues("error").Inc()
		return err
	}
	metrics.AIRAGUpdates.WithLabelValues("upserted").Inc()
	return nil
}

// decodeCatalogEvent accepts the normal object payload and the historical
// double-encoded form (a JSON string containing the object), so a consumer
// upgrade does not dead-letter the existing backlog.
func decodeCatalogEvent(data []byte) (catalogChangedEvent, error) {
	var ev catalogChangedEvent
	if err := json.Unmarshal(data, &ev); err == nil {
		return ev, nil
	}
	var wrapped string
	if err := json.Unmarshal(data, &wrapped); err == nil {
		if err := json.Unmarshal([]byte(wrapped), &ev); err == nil {
			return ev, nil
		}
	}
	return ev, fmt.Errorf("invalid catalog change payload")
}
