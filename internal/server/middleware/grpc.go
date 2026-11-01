package middleware

import (
	"context"
	"strings"

	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// publicMethods can be called without a JWT. DTM internal endpoints are
// network-isolated in production; they must never be exposed publicly.
var publicMethods = map[string]struct{}{
	"/user.v1.User/Register":                   {},
	"/user.v1.User/Login":                      {},
	"/api.product.v1.Product/GetProductDetail": {},
	"/api.product.v1.Product/ListProducts":     {},
	"/api.product.v1.Product/GetSku":           {},
	"/api.order.v1.Order/CreateOrderInner":     {},
	"/api.order.v1.Order/CancelOrder":          {},
	"/api.product.v1.Product/ReduceStock":      {},
	"/api.product.v1.Product/CompensateStock":  {},
	"/grpc.health.v1.Health/Check":             {},
	"/grpc.health.v1.Health/Watch":             {},
}

var adminMethods = map[string]struct{}{
	"/user.v1.User/UpdateRoleByID":          {},
	"/api.product.v1.Product/CreateProduct": {},
	"/api.product.v1.Product/UpdateProduct": {},
	"/api.product.v1.Product/SetOnSale":     {},
	"/api.product.v1.Product/AdjustStock":   {},
}

// GRPCAuth extracts a bearer token from gRPC metadata and injects typed claims.
func GRPCAuth(manager *auth.Manager) kratosmiddleware.Middleware {
	return func(handler kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			operation := operationFromContext(ctx)
			if _, ok := publicMethods[operation]; ok {
				return handler(ctx, req)
			}
			token := bearerFromMetadata(ctx)
			claims, err := manager.ParseBearer(token)
			if err != nil {
				return nil, err
			}
			ctx = auth.WithClaims(ctx, claims)
			if _, adminOnly := adminMethods[operation]; adminOnly && !auth.HasRole(ctx, "admin") {
				return nil, apperr.Forbidden("admin role required")
			}
			return handler(ctx, req)
		}
	}
}

// GRPCErrorMapper converts application errors to canonical gRPC status codes.
func GRPCErrorMapper() kratosmiddleware.Middleware {
	return func(handler kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := handler(ctx, req)
			if err == nil {
				return reply, nil
			}
			if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
				return reply, err
			}
			return reply, apperr.GRPCStatus(err)
		}
	}
}

func operationFromContext(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}

func bearerFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
