package ai

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// SearchKnowledge exposes retrieval for the admin quality harness. It is only
// wired through the admin BFF; the model never calls this path directly.
func (s *Service) SearchKnowledge(ctx context.Context, req *pb.KnowledgeSearchRequest) (*pb.KnowledgeSearchReply, error) {
	if s.rag == nil {
		return nil, apperr.Unavailable("rag is not configured", nil)
	}
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, apperr.Invalid("query is required")
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 5
	}
	if limit > 8 {
		limit = 8
	}
	docs, err := s.rag.Search(ctx, query, limit)
	if err != nil {
		return nil, apperr.Internal("search knowledge", err)
	}
	reply := &pb.KnowledgeSearchReply{Docs: make([]*pb.KnowledgeDoc, 0, len(docs))}
	for _, d := range docs {
		reply.Docs = append(reply.Docs, &pb.KnowledgeDoc{
			Id: d.ID, Title: d.Title, Content: d.Content, Kind: d.Kind, Score: d.Score,
		})
	}
	return reply, nil
}
