// Command catalog-stock-rebuild reconciles the catalog display projection
// (skus.stock/stock_version) from the authoritative inventory service.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	dsn := flag.String("dsn", env("CATALOG_MYSQL_DSN", ""), "catalog MySQL DSN")
	inventory := flag.String("inventory", env("INVENTORY_ADDR", "inventory:9003"), "inventory gRPC address")
	dry := flag.Bool("dry-run", false, "report differences without writing")
	flag.Parse()
	if *dsn == "" {
		fatal("catalog MySQL DSN is required")
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fatal(err.Error())
	}
	defer db.Close()
	conn, err := grpc.NewClient(*inventory, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fatal(err.Error())
	}
	defer conn.Close()
	client := pb.NewInventoryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	checked, mismatched, updated := 0, 0, 0
	for page := int32(1); ; page++ {
		list, err := client.ListStocks(ctx, &pb.ListStocksRequest{Page: page, PageSize: 200})
		if err != nil {
			fatal("list stocks: " + err.Error())
		}
		for _, st := range list.GetStocks() {
			// The deployment is single-warehouse; other warehouses are not
			// projected by the catalog consumer either.
			if st.GetWarehouseId() != 1 {
				continue
			}
			checked++
			var stock, version int64
			err := db.QueryRowContext(ctx, `SELECT stock, stock_version FROM skus WHERE id = ?`, st.GetSkuId()).Scan(&stock, &version)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				fatal("load sku: " + err.Error())
			}
			if stock == st.GetAvailable() && version == st.GetVersion() {
				continue
			}
			mismatched++
			if *dry {
				fmt.Printf("sku=%d catalog=(%d,%d) inventory=(%d,%d)\n", st.GetSkuId(), stock, version, st.GetAvailable(), st.GetVersion())
				continue
			}
			if _, err := db.ExecContext(ctx, `UPDATE skus SET stock = ?, stock_version = ? WHERE id = ?`,
				st.GetAvailable(), st.GetVersion(), st.GetSkuId()); err != nil {
				fatal("update projection: " + err.Error())
			}
			updated++
		}
		if int32(len(list.GetStocks())) < 200 || page*200 >= int32(list.GetTotal()) {
			break
		}
	}
	fmt.Printf("checked=%d mismatched=%d updated=%d dry_run=%v\n", checked, mismatched, updated, *dry)
}

func env(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
}

func fatal(msg string) { fmt.Fprintln(os.Stderr, msg); os.Exit(2) }
