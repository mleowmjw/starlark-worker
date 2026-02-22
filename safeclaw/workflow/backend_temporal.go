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
	ctx             temp.Context
	activityOptions *ActivityOptions
}

// NewTemporalBackend creates a new Temporal backend.
func NewTemporalBackend(ctx temp.Context) *TemporalBackend {
	return &TemporalBackend{
		ctx:             ctx,
		activityOptions: nil,
	}
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
	// Use stored activity options or defaults
	opts := temp.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	
	// Apply custom options if provided
	if b.activityOptions != nil {
		if b.activityOptions.StartToCloseTimeout > 0 {
			opts.StartToCloseTimeout = b.activityOptions.StartToCloseTimeout
		}
		if b.activityOptions.RetryPolicy != nil {
			opts.RetryPolicy = &temporal.RetryPolicy{
				MaximumAttempts: b.activityOptions.RetryPolicy.MaximumAttempts,
			}
			if b.activityOptions.RetryPolicy.InitialInterval > 0 {
				opts.RetryPolicy.InitialInterval = b.activityOptions.RetryPolicy.InitialInterval
			}
			if b.activityOptions.RetryPolicy.BackoffCoefficient > 0 {
				opts.RetryPolicy.BackoffCoefficient = b.activityOptions.RetryPolicy.BackoffCoefficient
			}
		}
	}
	
	ctx := temp.WithActivityOptions(b.ctx, opts)
	future := temp.ExecuteActivity(ctx, activity, args...)
	return &temporalFuture{future: future, ctx: b.ctx}
}

// WithActivityOptions returns a derived backend with the given activity options applied.
func (b *TemporalBackend) WithActivityOptions(opts ActivityOptions) Backend {
	return &TemporalBackend{
		ctx:             b.ctx,
		activityOptions: &opts,
	}
}

// IsReplaying returns true if the workflow is currently replaying.
func (b *TemporalBackend) IsReplaying() bool {
	return temp.IsReplaying(b.ctx)
}

// Go executes a function asynchronously using workflow.Go for deterministic execution.
func (b *TemporalBackend) Go(f func()) {
	temp.Go(b.ctx, func(ctx temp.Context) {
		f()
	})
}

// NewFuture creates a new future using Temporal's workflow.NewFuture.
func (b *TemporalBackend) NewFuture() (Future, Settable) {
	future, settable := temp.NewFuture(b.ctx)
	return &temporalFuture{future: future, ctx: b.ctx}, &temporalSettable{settable: settable}
}

// temporalSettable wraps Temporal's Settable.
type temporalSettable struct {
	settable temp.Settable
}

func (s *temporalSettable) Set(value any, err error) {
	s.settable.Set(value, err)
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
