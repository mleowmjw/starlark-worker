package workflow

import (
	"time"

	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	temp "go.temporal.io/sdk/workflow"
)

// TemporalBackend provides Temporal workflow-backed execution.
// Used in staging/production for deterministic, replay-safe operations.
type TemporalBackend struct {
	ctx temp.Context
}

// NewTemporalBackend creates a new Temporal backend.
func NewTemporalBackend(ctx temp.Context) *TemporalBackend {
	return &TemporalBackend{ctx: ctx}
}

var _ Backend = (*TemporalBackend)(nil)

// InWorkflow returns true for Temporal backend.
func (b *TemporalBackend) InWorkflow() bool {
	return true
}

// Now returns the current workflow time (deterministic).
func (b *TemporalBackend) Now() time.Time {
	return temp.Now(b.ctx)
}

// Sleep suspends workflow execution deterministically.
func (b *TemporalBackend) Sleep(d time.Duration) error {
	return temp.Sleep(b.ctx, d)
}

// SideEffect records non-deterministic operations for replay.
func (b *TemporalBackend) SideEffect(f func() any) EncodedValue {
	encoded := temp.SideEffect(b.ctx, func(ctx temp.Context) any {
		return f()
	})
	return &temporalEncodedValue{value: encoded}
}

// ExecuteActivity executes a Temporal activity asynchronously.
func (b *TemporalBackend) ExecuteActivity(activity any, args ...any) Future {
	// Set default activity options with timeouts
	ctx := temp.WithActivityOptions(b.ctx, temp.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})
	future := temp.ExecuteActivity(ctx, activity, args...)
	return &temporalFuture{future: future, ctx: b.ctx}
}

// temporalEncodedValue wraps Temporal's encoded value.
type temporalEncodedValue struct {
	value converter.EncodedValue
}

func (v *temporalEncodedValue) Get(valuePtr any) error {
	return v.value.Get(valuePtr)
}

// temporalFuture wraps Temporal's future.
type temporalFuture struct {
	future temp.Future
	ctx    temp.Context
}

func (f *temporalFuture) Get(valuePtr any) error {
	return f.future.Get(f.ctx, valuePtr)
}

func (f *temporalFuture) IsReady() bool {
	return f.future.IsReady()
}
