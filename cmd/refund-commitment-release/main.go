// Command refund-commitment-release is the AC-62/D-24 manual release entry.
// It only performs RESERVED -> RELEASED on one refund commitment; it never
// restores a permit, closes a group, changes the order lifecycle or calls
// Payment. The default is a read-only dry run; -apply writes the release row
// and the CAS state transition in one transaction.
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

	"gin-kratos-gorm/internal/services/order"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := flag.String("dsn", env("MYSQL_DSN", ""), "order MySQL DSN (or MYSQL_DSN)")
	refundID := flag.String("refund-id", "", "commitment refund_id")
	releaseID := flag.String("release-id", "", "idempotency key for this release")
	orderNo := flag.String("order-no", "", "optional expected order_no")
	groupNo := flag.String("group-no", "", "optional expected group_no")
	amount := flag.Int64("amount-cent", 0, "release amount; must equal the commitment amount")
	authorizationID := flag.String("authorization-id", "", "D-24 authorization id")
	operator := flag.String("operator", "", "operator")
	authorizedBy := flag.String("authorized-by", "", "authorized_by")
	reviewedBy := flag.String("reviewed-by", "", "second reviewer (required for amount >= 100000)")
	reason := flag.String("reason-code", "", "USER_WITHDRAW|BUSINESS_REJECT|FAILED_PERMANENT|MANUAL_ABANDON")
	evidence := flag.String("evidence-ref", "", "evidence reference")
	authorizedAt := flag.String("authorized-at", "", "RFC3339 authorization time")
	reviewedAt := flag.String("reviewed-at", "", "RFC3339 second-review time")
	expiresAt := flag.String("expires-at", "", "RFC3339 authorization expiry")
	apply := flag.Bool("apply", false, "write the release; default is a read-only dry run")
	timeout := flag.Duration("timeout", 30*time.Second, "overall timeout")
	flag.Parse()

	if strings.TrimSpace(*dsn) == "" {
		fatal("MYSQL_DSN (or -dsn) is required")
	}
	if *amount <= 0 {
		fatal("-amount-cent is required and must be positive")
	}
	authorized, err := parseRequiredTime("authorized-at", *authorizedAt)
	if err != nil {
		fatal(err.Error())
	}
	expires, err := parseRequiredTime("expires-at", *expiresAt)
	if err != nil {
		fatal(err.Error())
	}
	var reviewed time.Time
	if strings.TrimSpace(*reviewedAt) != "" {
		reviewed, err = time.Parse(time.RFC3339, strings.TrimSpace(*reviewedAt))
		if err != nil {
			fatal("-reviewed-at must be RFC3339")
		}
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	svc := order.NewWithOptions(db, nil, logger, nil, nil, nil, nil, order.Options{})
	result, err := svc.ReleaseRefundCommitment(ctx, order.RefundCommitmentReleaseRequest{
		ReleaseID: *releaseID, RefundID: *refundID, OrderNo: *orderNo, GroupNo: *groupNo,
		AmountCent: *amount, AuthorizationID: *authorizationID, Operator: *operator,
		AuthorizedBy: *authorizedBy, ReviewedBy: *reviewedBy, ReasonCode: *reason,
		EvidenceRef: *evidence, AuthorizedAt: authorized, ReviewedAt: reviewed,
		ExpiresAt: expires, DryRun: !*apply,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "release failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("dry_run=%v release_id=%s refund_id=%s amount_cent=%d state=%s version=%d already_released=%v\n",
		!*apply, result.ReleaseID, result.RefundID, result.AmountCent, result.State,
		result.Version, result.AlreadyReleased)
}

func parseRequiredTime(name, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("-%s is required (RFC3339)", name)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("-%s must be RFC3339", name)
	}
	return parsed, nil
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
