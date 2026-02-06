package concurrent

import (
	"context"
	"fmt"
	"sync"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
	"golang.org/x/sync/errgroup"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "concurrent"
}

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	return &Module{
		ctx: ctx,
	}
}

type Module struct {
	ctx context.Context
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "concurrent" }
func (f *Module) Type() string                          { return "concurrent" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: concurrent") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"run":          starlark.NewBuiltin("run", run),
	"batch_run":    starlark.NewBuiltin("batch_run", batchRun),
	"new_callable": starlark.NewBuiltin("new_callable", newCallable),
}

var properties = map[string]star.PropertyFactory{}

func run(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fn := args[0]
	callArgs := args[1:]

	// Create a future backed by a goroutine
	future := &Future{
		done: make(chan struct{}),
	}

	go func() {
		defer close(future.done)
		
		// Create a new thread for the goroutine
		subThread := &starlark.Thread{
			Name: "concurrent",
			Print: t.Print,
		}
		// Copy thread-local storage
		subThread.SetLocal("ctx", t.Local("ctx"))
		subThread.SetLocal("logger", t.Local("logger"))
		
		result, err := starlark.Call(subThread, fn, callArgs, kwargs)
		future.mu.Lock()
		future.result = result
		future.err = err
		future.mu.Unlock()
	}()

	return future, nil
}

func batchRun(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)
	ctx := safeclaw.GetContext(t) // Fix Bug 3: Get the actual execution context

	var callablesList *starlark.List
	var maxConcurrency int
	if err := starlark.UnpackArgs("batch_run", args, kwargs, "callables", &callablesList, "max_concurrency?", &maxConcurrency); err != nil {
		logger.Error("concurrent.batch_run: unpack args failed", "error", err)
		return nil, err
	}

	// Convert starlark.List to slice of callable objects
	callables := make([]*Callable, callablesList.Len())
	for i := 0; i < callablesList.Len(); i++ {
		item := callablesList.Index(i)
		if callableObj, ok := item.(*Callable); ok {
			callables[i] = callableObj
		} else {
			err := fmt.Errorf("list item %d is not a callable, got %T", i, item)
			logger.Error("concurrent.batch_run: invalid callable", "error", err)
			return nil, err
		}
	}

	// if maxConcurrency is not provided, start all jobs in parallel
	if maxConcurrency == 0 {
		maxConcurrency = len(callables)
	}

	// Create a batch future
	batchFuture := &BatchFuture{
		futures: make([]*Future, len(callables)),
		done:    make(chan struct{}),
	}

	// Use errgroup for controlled concurrency with the actual context
	g, gCtx := errgroup.WithContext(ctx) // Fix Bug 3: Use the actual context for cancellation propagation
	g.SetLimit(maxConcurrency)

	for i, callableObj := range callables {
		i := i
		callableObj := callableObj
		
		future := &Future{
			done: make(chan struct{}),
		}
		batchFuture.futures[i] = future

		g.Go(func() error {
			defer close(future.done)
			
			// Check if context is cancelled before starting work
			if err := gCtx.Err(); err != nil {
				future.mu.Lock()
				future.err = err
				future.mu.Unlock()
				return nil
			}
			
			// Create a new thread for the goroutine
			subThread := &starlark.Thread{
				Name: "concurrent",
				Print: t.Print,
			}
			// Use the errgroup context for cancellation propagation
			subThread.SetLocal("ctx", gCtx)
			subThread.SetLocal("logger", t.Local("logger"))
			
			result, err := starlark.Call(subThread, callableObj.Fn, callableObj.Args, nil)
			future.mu.Lock()
			future.result = result
			future.err = err
			future.mu.Unlock()
			
			return nil // We store errors in the future, not in errgroup
		})
	}

	// Wait for all goroutines to complete in a separate goroutine
	go func() {
		g.Wait()
		close(batchFuture.done)
	}()

	return batchFuture, nil
}

func newCallable(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if args.Len() < 1 {
		return nil, fmt.Errorf("new_callable requires at least 1 argument")
	}
	
	fn := args[0]
	callArgs := args[1:]
	
	return &Callable{
		Fn:   fn,
		Args: callArgs,
	}, nil
}

// Future represents an asynchronous result
type Future struct {
	mu     sync.Mutex
	done   chan struct{}
	result starlark.Value
	err    error
}

var _ starlark.Value = &Future{}
var _ starlark.HasAttrs = &Future{}

func (f *Future) String() string        { return "<Future>" }
func (f *Future) Type() string          { return "Future" }
func (f *Future) Freeze()                {}
func (f *Future) Truth() starlark.Bool  { return true }
func (f *Future) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Future") }

func (f *Future) Attr(name string) (starlark.Value, error) {
	switch name {
	case "result":
		return starlark.NewBuiltin("result", f.resultMethod), nil
	case "is_ready":
		return starlark.NewBuiltin("is_ready", f.isReadyMethod), nil
	default:
		return nil, nil
	}
}

func (f *Future) AttrNames() []string {
	return []string{"result", "is_ready"}
}

func (f *Future) resultMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Wait for the future to complete
	<-f.done
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *Future) isReadyMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	select {
	case <-f.done:
		return starlark.True, nil
	default:
		return starlark.False, nil
	}
}

// BatchFuture represents multiple futures
type BatchFuture struct {
	futures []*Future
	done    chan struct{}
}

var _ starlark.Value = &BatchFuture{}
var _ starlark.HasAttrs = &BatchFuture{}

func (b *BatchFuture) String() string        { return "<BatchFuture>" }
func (b *BatchFuture) Type() string          { return "BatchFuture" }
func (b *BatchFuture) Freeze()                {}
func (b *BatchFuture) Truth() starlark.Bool  { return true }
func (b *BatchFuture) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: BatchFuture") }

func (b *BatchFuture) Attr(name string) (starlark.Value, error) {
	switch name {
	case "result":
		return starlark.NewBuiltin("result", b.resultMethod), nil
	case "is_ready":
		return starlark.NewBuiltin("is_ready", b.isReadyMethod), nil
	default:
		return nil, nil
	}
}

func (b *BatchFuture) AttrNames() []string {
	return []string{"result", "is_ready"}
}

func (b *BatchFuture) resultMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Wait for all futures to complete
	<-b.done
	
	results := make([]starlark.Value, len(b.futures))
	for i, future := range b.futures {
		<-future.done
		future.mu.Lock()
		if future.err != nil {
			future.mu.Unlock()
			return nil, future.err
		}
		results[i] = future.result
		future.mu.Unlock()
	}
	
	return starlark.NewList(results), nil
}

func (b *BatchFuture) isReadyMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	select {
	case <-b.done:
		return starlark.True, nil
	default:
		return starlark.False, nil
	}
}

// Callable wraps a function and its arguments
type Callable struct {
	Fn   starlark.Value
	Args starlark.Tuple
}

var _ starlark.Value = &Callable{}

func (c *Callable) String() string        { return "<Callable>" }
func (c *Callable) Type() string          { return "Callable" }
func (c *Callable) Freeze()                {}
func (c *Callable) Truth() starlark.Bool  { return true }
func (c *Callable) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Callable") }
