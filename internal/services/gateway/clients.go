package gateway

import (
	"context"
	"fmt"
	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/grpcx"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	// requestTimeout bounds one BFF request, including every downstream call.
	requestTimeout = 15 * time.Second
	// defaultPaymentChannel is used when the client omits "channel" on the
	// simulated payment callback.
	defaultPaymentChannel = "MOCK"
	// paymentSucceeded is the only callback status exposed by this route.
	paymentSucceeded = "SUCCEEDED"
)

// serviceAddrs are the direct gRPC addresses used when no registry is
// configured. They mirror the port table in docs/contracts.md and can be
// overridden with <SERVICE>_ADDR (e.g. IDENTITY_ADDR).

// serviceAddrs are the direct gRPC addresses used when no registry is
// configured. They mirror the port table in docs/contracts.md and can be
// overridden with <SERVICE>_ADDR (e.g. IDENTITY_ADDR).
var serviceAddrs = map[string]string{
	"identity":    "identity:9001",
	"catalog":     "catalog:9002",
	"inventory":   "inventory:9003",
	"cart":        "cart:9004",
	"pricing":     "pricing:9005",
	"order":       "order:9006",
	"payment":     "payment:9007",
	"fulfillment": "fulfillment:9008",
	"aftersales":  "aftersales:9009",
	"seckill":     "seckill:9010",
}

// Clients bundles every downstream gRPC client used by the public BFF.

// Clients bundles every downstream gRPC client used by the public BFF.
type Clients struct {
	Identity    pb.IdentityServiceClient
	Catalog     pb.CatalogServiceClient
	Inventory   pb.InventoryServiceClient
	Cart        pb.CartServiceClient
	Pricing     pb.PricingServiceClient
	Order       pb.OrderServiceClient
	Payment     pb.PaymentServiceClient
	Fulfillment pb.FulfillmentServiceClient
	AfterSales  pb.AfterSalesServiceClient
	Seckill     pb.SeckillServiceClient

	conns map[string]*grpc.ClientConn
}

// Close releases every client connection.

// Close releases every client connection.
func (c *Clients) Close() error {
	var first error
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// States reports the gRPC connectivity state of every downstream client.

// States reports the gRPC connectivity state of every downstream client.
func (c *Clients) States() map[string]string {
	out := make(map[string]string, len(c.conns))
	for name, conn := range c.conns {
		if conn == nil {
			continue
		}
		out[name] = conn.GetState().String()
	}
	return out
}

// Dial opens the downstream client connections using Kubernetes Service DNS or
// <SERVICE>_ADDR overrides.

// Dial opens the downstream client connections using Kubernetes Service DNS or
// <SERVICE>_ADDR overrides.
func Dial(ctx context.Context) (*Clients, error) {
	clients := &Clients{conns: map[string]*grpc.ClientConn{}}
	targets := []struct {
		name string
		bind func(*grpc.ClientConn)
	}{
		{"identity", func(conn *grpc.ClientConn) { clients.Identity = pb.NewIdentityServiceClient(conn) }},
		{"catalog", func(conn *grpc.ClientConn) { clients.Catalog = pb.NewCatalogServiceClient(conn) }},
		{"inventory", func(conn *grpc.ClientConn) { clients.Inventory = pb.NewInventoryServiceClient(conn) }},
		{"cart", func(conn *grpc.ClientConn) { clients.Cart = pb.NewCartServiceClient(conn) }},
		{"pricing", func(conn *grpc.ClientConn) { clients.Pricing = pb.NewPricingServiceClient(conn) }},
		{"order", func(conn *grpc.ClientConn) { clients.Order = pb.NewOrderServiceClient(conn) }},
		{"payment", func(conn *grpc.ClientConn) { clients.Payment = pb.NewPaymentServiceClient(conn) }},
		{"fulfillment", func(conn *grpc.ClientConn) { clients.Fulfillment = pb.NewFulfillmentServiceClient(conn) }},
		{"aftersales", func(conn *grpc.ClientConn) { clients.AfterSales = pb.NewAfterSalesServiceClient(conn) }},
		{"seckill", func(conn *grpc.ClientConn) { clients.Seckill = pb.NewSeckillServiceClient(conn) }},
	}
	for _, target := range targets {
		conn, err := grpcx.Dial(ctx, targetFor(target.name))
		if err != nil {
			_ = clients.Close()
			return nil, fmt.Errorf("dial %s: %w", target.name, err)
		}
		clients.conns[target.name] = conn
		target.bind(conn)
	}
	return clients, nil
}

// targetFor returns the gRPC target for one downstream service.

// targetFor returns the gRPC target for one downstream service.
func targetFor(name string) string {
	if addr := strings.TrimSpace(os.Getenv(strings.ToUpper(name) + "_ADDR")); addr != "" {
		return addr
	}
	if addr, ok := serviceAddrs[name]; ok {
		return addr
	}
	return name + ":9000"
}

// NewEngine builds the public gateway Gin engine. The routes under /api/v1 that
// are not explicitly public require a valid bearer token.
// Options toggles BFF capabilities that must not be on in production.
