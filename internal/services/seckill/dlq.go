package seckill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// ReplayDLQ re-publishes stored DLQ commands for qualifications that still
// retain their inventory hold. It is deliberately operator-triggered; a
// permanent failure can be retried after the upstream problem is fixed.
func (s *Service) ReplayDLQ(ctx context.Context, req *pb.ReplaySeckillDLQRequest) (*pb.ReplaySeckillDLQReply, error) {
	if s.mq == nil || s.mq.JS == nil {
		return nil, apperr.Unavailable("nats is not configured", fmt.Errorf("jetstream client is nil"))
	}
	info, err := s.mq.JS.StreamInfo(seckillStream)
	if err != nil {
		return nil, apperr.Unavailable("read seckill stream", err)
	}
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	reply := &pb.ReplaySeckillDLQReply{}
	var errs []string
	for seq := info.State.FirstSeq; seq <= info.State.LastSeq && int(reply.Replayed+reply.Skipped) < limit; seq++ {
		raw, err := s.mq.JS.GetMsg(seckillStream, seq)
		if err != nil || raw == nil || !strings.HasPrefix(raw.Subject, "seckill.order.dlq.") {
			continue
		}
		if req.GetActivityId() > 0 && !strings.HasSuffix(raw.Subject, fmt.Sprintf(".%d", req.GetActivityId())) {
			continue
		}
		var envelope struct {
			Command orderCommand `json:"command"`
		}
		if err := json.Unmarshal(raw.Data, &envelope); err != nil || envelope.Command.TokenHash == "" {
			reply.Skipped++
			continue
		}
		cmd := envelope.Command
		q, err := s.getQualification(ctx, cmd.TokenHash, cmd.UserID)
		if err != nil || q.status == "CANCELLED" || q.status == "EXPIRED" || q.status == "ORDERED" {
			reply.Skipped++
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE seckill_qualifications
			SET status='ISSUED', published_at=NULL WHERE token_hash=? AND status='FAILED'`, cmd.TokenHash); err != nil {
			errs = append(errs, fmt.Sprintf("token %s: %v", cmd.TokenHash, err))
			reply.Skipped++
			continue
		}
		a := &activity{id: cmd.ActivityID, skuID: cmd.SKU, priceCent: cmd.PriceCent}
		q.status = "ISSUED"
		q.publishedAt.Valid = false
		if err := s.publishQualification(ctx, q, a); err != nil {
			errs = append(errs, fmt.Sprintf("token %s: %v", cmd.TokenHash, err))
			reply.Skipped++
			continue
		}
		if err := s.mq.JS.DeleteMsg(seckillStream, seq); err != nil {
			errs = append(errs, fmt.Sprintf("erase %d: %v", seq, err))
		}
		reply.Replayed++
	}
	reply.Errors = strings.Join(errs, "; ")
	return reply, nil
}
