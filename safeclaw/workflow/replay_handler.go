package workflow

import (
	"context"
	"log/slog"
)

// ReplayAwareHandler wraps an slog.Handler and suppresses log output during workflow replay.
// This prevents duplicate logs when Temporal workflows are replayed.
type ReplayAwareHandler struct {
	inner       slog.Handler
	isReplaying func() bool
}

// NewReplayAwareHandler creates a new replay-aware slog handler.
func NewReplayAwareHandler(inner slog.Handler, isReplaying func() bool) *ReplayAwareHandler {
	return &ReplayAwareHandler{
		inner:       inner,
		isReplaying: isReplaying,
	}
}

// Enabled returns false during replay to suppress log output.
func (h *ReplayAwareHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.isReplaying != nil && h.isReplaying() {
		return false
	}
	return h.inner.Enabled(ctx, level)
}

// Handle delegates to the inner handler if not replaying.
func (h *ReplayAwareHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.isReplaying != nil && h.isReplaying() {
		return nil
	}
	return h.inner.Handle(ctx, record)
}

// WithAttrs returns a new handler with additional attributes.
func (h *ReplayAwareHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ReplayAwareHandler{
		inner:       h.inner.WithAttrs(attrs),
		isReplaying: h.isReplaying,
	}
}

// WithGroup returns a new handler with a group name.
func (h *ReplayAwareHandler) WithGroup(name string) slog.Handler {
	return &ReplayAwareHandler{
		inner:       h.inner.WithGroup(name),
		isReplaying: h.isReplaying,
	}
}
