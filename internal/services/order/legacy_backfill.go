package order

import (
	"context"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// LegacyBackfillOptions controls the T2.5 historical PAID backfill. The default
// is a dry run: no order, queue, refund or sequence row is written. Apply is an
// explicit opt-in after T2.1 producers are deployed and a dry run has been
// reconciled with the ops count.
type LegacyBackfillOptions struct {
	DryRun  bool
	Limit   int
	OrderNo string
}

// LegacyBackfillReport is the dry-run/apply reconciliation. Scanned ==
// PAIDEmpty for the selected window. Unique/Zero/Ambiguous are the dry-run
// classification; Resolved/Queued/Skipped are the apply outcome. Remaining is
// the post-apply count of PAID orders that still have no settlement identity
// (ambiguous rows are expected to remain until a human resolves them).
type LegacyBackfillReport struct {
	Scanned       int
	PAIDEmpty     int
	Unique        int
	Zero          int
	Ambiguous     int
	NoCaptured    int
	Resolved      int
	Queued        int
	Skipped       int
	Errors        int
	Remaining     int
	Samples       []string
	ErrorMessages []string
}

type legacyBackfillKind int

const (
	legacyBackfillSkipped legacyBackfillKind = iota
	legacyBackfillUnique
	legacyBackfillZero
	legacyBackfillAmbiguous
)

func (k legacyBackfillKind) String() string {
	switch k {
	case legacyBackfillUnique:
		return "unique"
	case legacyBackfillZero:
		return "zero"
	case legacyBackfillAmbiguous:
		return "ambiguous"
	default:
		return "skipped"
	}
}

type legacyBackfillPreview struct {
	Kind           legacyBackfillKind
	CandidateCount int
	AnyCaptured    bool
}

// BackfillLegacyPAID resolves historical PAID orders whose effective payment
// is still empty. It is deliberately a separate entry point from the T2.4 SLA
// worker: this tool is the one-time T2.5 backfill, while the worker owns later
// expiry/terminal transitions.
//
// "Only PAID" is enforced in SQL. A dry run is read-only and classifies each
// row without calling Payment.Refund; apply calls ensureLegacyResolution, which
// is idempotent (already-settled/queued rows are read back, not rewritten).
func (s *Service) BackfillLegacyPAID(ctx context.Context, opts LegacyBackfillOptions) (LegacyBackfillReport, error) {
	var report LegacyBackfillReport
	if s == nil || s.db == nil {
		return report, apperr.Internal("order service db is required for legacy backfill", nil)
	}
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}
	orders, err := s.legacyBackfillCandidates(ctx, opts)
	if err != nil {
		return report, err
	}
	report.Scanned = len(orders)
	report.PAIDEmpty = len(orders)
	var firstErr error
	for _, orderNo := range orders {
		if opts.DryRun {
			preview, perr := s.previewLegacyResolution(ctx, orderNo)
			if perr != nil {
				report.Errors++
				report.ErrorMessages = appendBounded(report.ErrorMessages, fmt.Sprintf("%s: %v", orderNo, perr))
				if firstErr == nil {
					firstErr = perr
				}
				continue
			}
			switch preview.Kind {
			case legacyBackfillUnique:
				report.Unique++
			case legacyBackfillZero:
				report.Zero++
			case legacyBackfillAmbiguous:
				report.Ambiguous++
			default:
				report.Skipped++
			}
			if !preview.AnyCaptured {
				report.NoCaptured++
			}
			report.Samples = appendBounded(report.Samples,
				fmt.Sprintf("%s kind=%s candidates=%d captured=%v", orderNo, preview.Kind, preview.CandidateCount, preview.AnyCaptured))
			continue
		}

		res, rerr := s.ensureLegacyResolution(ctx, orderNo)
		if rerr != nil {
			report.Errors++
			report.ErrorMessages = appendBounded(report.ErrorMessages, fmt.Sprintf("%s: %v", orderNo, rerr))
			if firstErr == nil {
				firstErr = rerr
			}
			continue
		}
		switch res.State {
		case legacyResolutionStateResolved:
			report.Resolved++
		case legacyResolutionStateExpiredZero:
			report.Zero++
		case legacyResolutionStateOpen, legacyResolutionStateResolving, legacyResolutionStateCompensating:
			report.Queued++
		default:
			report.Skipped++
		}
		report.Samples = appendBounded(report.Samples, fmt.Sprintf("%s state=%s payment=%s", orderNo, res.State, res.PaymentNo))
	}
	if !opts.DryRun {
		remaining, cerr := s.countLegacyPAIDEmpty(ctx, opts)
		if cerr != nil {
			if firstErr == nil {
				firstErr = cerr
			}
			report.ErrorMessages = appendBounded(report.ErrorMessages, fmt.Sprintf("count remaining: %v", cerr))
		} else {
			report.Remaining = remaining
		}
	}
	return report, firstErr
}

// legacyBackfillCandidates selects only PAID orders whose effective payment is
// empty. A caller-scoped order_no is used by ops for a single-row retry; the
// limit keeps Payment calls bounded.
func (s *Service) legacyBackfillCandidates(ctx context.Context, opts LegacyBackfillOptions) ([]string, error) {
	query := `SELECT order_no FROM orders
		WHERE status = ? AND settlement_payment_no = ''`
	args := []any{statusPaid}
	if orderNo := strings.TrimSpace(opts.OrderNo); orderNo != "" {
		query += ` AND order_no = ?`
		args = append(args, orderNo)
	}
	query += ` ORDER BY updated_at, order_no LIMIT ?`
	args = append(args, opts.Limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperr.Internal("scan legacy PAID candidates", err)
	}
	defer rows.Close()
	orders := make([]string, 0, opts.Limit)
	for rows.Next() {
		var orderNo string
		if err := rows.Scan(&orderNo); err != nil {
			return nil, apperr.Internal("scan legacy PAID candidate row", err)
		}
		orders = append(orders, orderNo)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate legacy PAID candidates", err)
	}
	return orders, nil
}

func (s *Service) countLegacyPAIDEmpty(ctx context.Context, opts LegacyBackfillOptions) (int, error) {
	query := `SELECT COUNT(*) FROM orders WHERE status = ? AND settlement_payment_no = ''`
	args := []any{statusPaid}
	if orderNo := strings.TrimSpace(opts.OrderNo); orderNo != "" {
		query += ` AND order_no = ?`
		args = append(args, orderNo)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, apperr.Internal("count legacy PAID candidates", err)
	}
	return n, nil
}

// previewLegacyResolution is the read-only twin of ensureLegacyResolution. It
// must keep the same candidate rule: only SUCCEEDED intents whose captured
// amount equals the order pay_cent are candidates; a zero-pay order with a
// captured intent is never zero. T2.7 asserts that dry-run counts match apply
// outcomes for the same fixture.
func (s *Service) previewLegacyResolution(ctx context.Context, orderNo string) (legacyBackfillPreview, error) {
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return legacyBackfillPreview{}, apperr.Internal("load order for legacy preview", err)
	}
	if record == nil || record.Status != statusPaid || strings.TrimSpace(record.SettlementPaymentNo) != "" {
		return legacyBackfillPreview{Kind: legacyBackfillSkipped}, nil
	}
	if s.payments == nil {
		return legacyBackfillPreview{}, apperr.Unavailable("payment client is required for legacy backfill preview", nil)
	}
	intents, err := s.payments.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: orderNo})
	if err != nil {
		return legacyBackfillPreview{}, apperr.Unavailable("list intents for legacy backfill preview", err)
	}
	preview := legacyBackfillPreview{}
	candidates := 0
	for _, in := range s.legacyIntentsForOrder(orderNo, intents.GetIntents()) {
		if in.GetCapturedCent() > 0 {
			preview.AnyCaptured = true
		}
		if in.GetStatus() == "SUCCEEDED" && in.GetCapturedCent() == record.PayCent {
			candidates++
		}
	}
	preview.CandidateCount = candidates
	switch {
	case record.PayCent == 0 && !preview.AnyCaptured:
		preview.Kind = legacyBackfillZero
	case candidates == 1:
		preview.Kind = legacyBackfillUnique
	default:
		preview.Kind = legacyBackfillAmbiguous
	}
	return preview, nil
}

func appendBounded(list []string, value string) []string {
	const max = 10
	if len(list) >= max {
		return list
	}
	return append(list, value)
}
