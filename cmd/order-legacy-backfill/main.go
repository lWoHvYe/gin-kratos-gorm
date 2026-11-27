// Command order-legacy-backfill is the T2.5 backfill for historical PAID
// orders whose effective payment is still empty. It only selects PAID orders,
// defaults to a read-only dry run, and delegates the actual resolution to the
// same ensureLegacyResolution path used by the service.
//
// Run it only after the T2.1 producer fix is deployed. First run the default
// dry run, reconcile the counts with ops, then re-run with -apply. Apply is
// idempotent: already-settled orders leave the PAID-empty window and ambiguous
// rows are upserted into legacy_payment_resolution, not duplicated.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/services/order"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	dsn := flag.String("dsn", env("MYSQL_DSN", ""), "order MySQL DSN (or MYSQL_DSN)")
	payment := flag.String("payment", env("PAYMENT_ADDR", "127.0.0.1:9007"), "payment gRPC address")
	apply := flag.Bool("apply", false, "write resolutions; default is a read-only dry run. Requires the T2.1 producer fix and an accepted dry-run")
	limit := flag.Int("limit", 1000, "max PAID-empty orders per run")
	orderNo := flag.String("order-no", "", "limit to one order (ops retry)")
	timeout := flag.Duration("timeout", 5*time.Minute, "overall command timeout")
	flag.Parse()

	if strings.TrimSpace(*dsn) == "" {
		fatal("MYSQL_DSN (or -dsn) is required")
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fatal(err.Error())
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fatal("ping order DB: " + err.Error())
	}
	conn, err := grpc.NewClient(*payment, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fatal("payment client: " + err.Error())
	}
	defer func() { _ = conn.Close() }()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := order.NewWithOptions(db, nil, logger, nil, nil, nil, nil, order.Options{})
	svc.EnableSettlementClients(pb.NewPaymentServiceClient(conn), nil)

	report, runErr := svc.BackfillLegacyPAID(ctx, order.LegacyBackfillOptions{
		DryRun:  !*apply,
		Limit:   *limit,
		OrderNo: *orderNo,
	})
	fmt.Printf("dry_run=%v scanned=%d paid_empty=%d unique=%d zero=%d ambiguous=%d no_captured=%d resolved=%d queued=%d skipped=%d errors=%d remaining=%d\n",
		!*apply, report.Scanned, report.PAIDEmpty, report.Unique, report.Zero,
		report.Ambiguous, report.NoCaptured, report.Resolved, report.Queued, report.Skipped,
		report.Errors, report.Remaining)
	for _, sample := range report.Samples {
		fmt.Printf("sample %s\n", sample)
	}
	for _, message := range report.ErrorMessages {
		fmt.Fprintf(os.Stderr, "error %s\n", message)
	}
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "backfill failed: %v\n", runErr)
	}
	if runErr != nil || report.Errors > 0 {
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
