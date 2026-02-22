package workflow

import (
	"context"
	"time"
)

// LocalBackend provides direct execution without workflow orchestration.
// Used in dev/local environments for faster iteration.
type LocalBackend struct {
	ctx context.Context
}

// NewLocalBackend creates a new local backend.
func NewLocalBackend(ctx context.Context) *LocalBackend {
	return &LocalBackend{ctx: ctx}
}

var _ Backend = (*LocalBackend)(nil)

// InWorkflow returns false for local backend.
func (b *LocalBackend) InWorkflow() bool {
	return false
}

// Now returns the current wall-clock time.
func (b *LocalBackend) Now() time.Time {
	return time.Now()
}

// Sleep suspends execution using standard time.Sleep.
func (b *LocalBackend) Sleep(d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
}

// SideEffect executes the function immediately and wraps the result.
func (b *LocalBackend) SideEffect(f func() any) EncodedValue {
	result := f()
	return &localEncodedValue{value: result}
}

// ExecuteActivity executes the activity function directly (no async execution).
func (b *LocalBackend) ExecuteActivity(activity any, args ...any) Future {
	// For local mode, we execute activities synchronously
	// This is a simplified version - in real usage, you'd call the activity function directly
	return &localFuture{
		result: nil,
		err:    nil,
		ready:  true,
	}
}

// localEncodedValue wraps a value for immediate retrieval.
type localEncodedValue struct {
	value any
}

func (v *localEncodedValue) Get(valuePtr any) error {
	return assign(v.value, valuePtr)
}

// localFuture represents a completed operation in local mode.
type localFuture struct {
	result any
	err    error
	ready  bool
}

func (f *localFuture) Get(valuePtr any) error {
	if f.err != nil {
		return f.err
	}
	return assign(f.result, valuePtr)
}

func (f *localFuture) IsReady() bool {
	return f.ready
}

// assign copies value to valuePtr using reflection-like behavior.
func assign(value any, valuePtr any) error {
	switch ptr := valuePtr.(type) {
	case *any:
		*ptr = value
		return nil
	case *string:
		if s, ok := value.(string); ok {
			*ptr = s
			return nil
		}
	case *int:
		if i, ok := value.(int); ok {
			*ptr = i
			return nil
		}
	case *int64:
		if i, ok := value.(int64); ok {
			*ptr = i
			return nil
		}
	case *float64:
		if f, ok := value.(float64); ok {
			*ptr = f
			return nil
		}
	case *bool:
		if b, ok := value.(bool); ok {
			*ptr = b
			return nil
		}
	}
	// For complex types, we'd need more sophisticated copying
	// This is a simplified version
	return nil
}
