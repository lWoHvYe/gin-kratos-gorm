package inventory

import (
	"context"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// ReplayInventoryOperationForCLI keeps the one-shot CLI entrypoint stable.
func (s *Service) ReplayInventoryOperationForCLI(ctx context.Context, operationID string, skus []int64) error {
	return s.replayOperation(ctx, operationID, skus)
}

// ReplayInventoryOperation is the gRPC/admin recovery surface over the same
// explicit replay. Completed items are never replayed.
func (s *Service) ReplayInventoryOperation(ctx context.Context, req *pb.ReplayInventoryOperationRequest) (*pb.Empty, error) {
	if err := s.replayOperation(ctx, req.GetOperationId(), req.GetSkuIds()); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
