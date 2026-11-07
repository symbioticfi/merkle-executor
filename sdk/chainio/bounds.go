package chainio

import "context"

// ReadBounds is the scheduler's immutable transport contract for one partition.
// Backends must preserve the context through their RPC transport and enforce the
// exact encoded request, raw response, and post-codec calldata bounds before any
// result reaches a consumer. A smaller bound never causes an implicit fallback.
type ReadBounds struct {
	MaxRequestBytes  uint64
	MaxResponseBytes uint64
	MaxCalldataBytes uint64
}
type readBoundsKey struct{}

// WithReadBounds carries explicit resource bounds through an existing BatchReader.
func WithReadBounds(ctx context.Context, bounds ReadBounds) context.Context {
	return context.WithValue(ctx, readBoundsKey{}, bounds)
}
func ReadBoundsFromContext(ctx context.Context) (ReadBounds, bool) {
	bounds, ok := ctx.Value(readBoundsKey{}).(ReadBounds)
	return bounds, ok
}
