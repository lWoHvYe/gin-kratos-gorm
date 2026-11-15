package ai

import "context"

// userIDKey carries the authenticated user from the gRPC handler into tools.
// Tools must never take a user id from model-generated arguments.
type userIDKey struct{}

func withUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok && id > 0
}
