package workflow

import (
	"context"
	"fmt"
	"reflect"
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

// ExecuteActivity executes the activity function directly via reflection.
// Activities are called synchronously with the context as the first argument.
func (b *LocalBackend) ExecuteActivity(activity any, args ...any) Future {
	fn := reflect.ValueOf(activity)
	fnType := fn.Type()

	// Validate it's a function
	if fnType.Kind() != reflect.Func {
		return &localFuture{
			result: nil,
			err:    fmt.Errorf("activity must be a function, got %T", activity),
			ready:  true,
		}
	}

	// Build call arguments: context.Context + args
	callArgs := []reflect.Value{reflect.ValueOf(b.ctx)}
	for _, arg := range args {
		callArgs = append(callArgs, reflect.ValueOf(arg))
	}

	// Call the activity function
	results := fn.Call(callArgs)

	// Extract (result, error) from return values
	// Activities typically return (T, error) or just error
	var result any
	var err error

	if len(results) == 0 {
		// No return values
		return &localFuture{result: nil, err: nil, ready: true}
	}

	// Last return value should be error (or nil)
	lastVal := results[len(results)-1]
	if lastVal.IsValid() && !lastVal.IsNil() {
		if e, ok := lastVal.Interface().(error); ok {
			err = e
		}
	}

	// If there's a non-error return value, it's the result
	if len(results) >= 2 {
		result = results[0].Interface()
	}

	return &localFuture{
		result: result,
		err:    err,
		ready:  true,
	}
}

// WithActivityOptions returns self (no-op for local backend).
// Activity options are not relevant for direct execution.
func (b *LocalBackend) WithActivityOptions(opts ActivityOptions) Backend {
	return b
}

// IsReplaying always returns false for local backend.
func (b *LocalBackend) IsReplaying() bool {
	return false
}

// Go executes a function asynchronously using a standard goroutine.
func (b *LocalBackend) Go(f func()) {
	go f()
}

// NewFuture creates a new future backed by a channel.
func (b *LocalBackend) NewFuture() (Future, Settable) {
	future := &localManualFuture{
		done: make(chan struct{}),
	}
	return future, future
}

// localManualFuture is a future that can be manually fulfilled.
type localManualFuture struct {
	done   chan struct{}
	result any
	err    error
}

func (f *localManualFuture) Get(valuePtr any) error {
	<-f.done
	if f.err != nil {
		return f.err
	}
	return assign(f.result, valuePtr)
}

func (f *localManualFuture) IsReady() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

func (f *localManualFuture) Set(value any, err error) {
	f.result = value
	f.err = err
	close(f.done)
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

// assign copies value to valuePtr using reflection for robust type handling.
func assign(value any, valuePtr any) error {
	if valuePtr == nil {
		return fmt.Errorf("valuePtr is nil")
	}

	ptrVal := reflect.ValueOf(valuePtr)
	if ptrVal.Kind() != reflect.Pointer {
		return fmt.Errorf("valuePtr must be a pointer, got %T", valuePtr)
	}

	if ptrVal.IsNil() {
		return fmt.Errorf("valuePtr is nil pointer")
	}

	// Handle nil value
	if value == nil {
		// Set to zero value
		ptrVal.Elem().Set(reflect.Zero(ptrVal.Elem().Type()))
		return nil
	}

	srcVal := reflect.ValueOf(value)
	dstType := ptrVal.Elem().Type()

	// Direct assignment if types match
	if srcVal.Type().AssignableTo(dstType) {
		ptrVal.Elem().Set(srcVal)
		return nil
	}

	// Try conversion if types are convertible
	if srcVal.Type().ConvertibleTo(dstType) {
		ptrVal.Elem().Set(srcVal.Convert(dstType))
		return nil
	}

	return fmt.Errorf("cannot assign %T to %T", value, valuePtr)
}
