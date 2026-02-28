package concurrent

import (
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "concurrent"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	backend := workflow.GetBackend(ctx)
	return &Module{
		backend: backend,
	}
}

type Module struct {
	backend workflow.Backend
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

func run(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	receiver := fn.Receiver().(*Module)
	callFn := args[0]
	callArgs := args[1:]

	// Create a future using the backend
	future, settable := receiver.backend.NewFuture()

	// Execute asynchronously using backend.Go
	receiver.backend.Go(func() {
		// Create a new thread for the concurrent execution
		subThread := &starlark.Thread{
			Name:  "concurrent",
			Print: t.Print,
		}
		// Copy thread-local storage
		subThread.SetLocal("ctx", t.Local("ctx"))
		subThread.SetLocal("logger", t.Local("logger"))
		subThread.SetLocal("workflow_backend", t.Local("workflow_backend"))

		result, err := starlark.Call(subThread, callFn, callArgs, kwargs)
		settable.Set(result, err)
	})

	return &Future{future: future}, nil
}

func batchRun(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)
	receiver := fn.Receiver().(*Module)

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

	// Create futures for each callable
	futures := make([]workflow.Future, len(callables))
	settables := make([]workflow.Settable, len(callables))
	for i := range callables {
		futures[i], settables[i] = receiver.backend.NewFuture()
	}

	// Semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrency)

	// Launch goroutines using backend.Go
	for i, callableObj := range callables {

		receiver.backend.Go(func() {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create a new thread for the concurrent execution
			subThread := &starlark.Thread{
				Name:  "concurrent",
				Print: t.Print,
			}
			// Copy thread-local storage
			subThread.SetLocal("ctx", t.Local("ctx"))
			subThread.SetLocal("logger", t.Local("logger"))
			subThread.SetLocal("workflow_backend", t.Local("workflow_backend"))

			result, err := starlark.Call(subThread, callableObj.Fn, callableObj.Args, nil)
			settables[i].Set(result, err)
		})
	}

	// Wrap futures in BatchFuture
	return &BatchFuture{futures: futures}, nil
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

// Future wraps a workflow.Future for Starlark access.
type Future struct {
	future workflow.Future
}

var _ starlark.Value = &Future{}
var _ starlark.HasAttrs = &Future{}

func (f *Future) String() string        { return "<Future>" }
func (f *Future) Type() string          { return "Future" }
func (f *Future) Freeze()               {}
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
	var result starlark.Value
	if err := f.future.Get(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (f *Future) isReadyMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if f.future.IsReady() {
		return starlark.True, nil
	}
	return starlark.False, nil
}

// BatchFuture represents multiple futures
type BatchFuture struct {
	futures []workflow.Future
}

var _ starlark.Value = &BatchFuture{}
var _ starlark.HasAttrs = &BatchFuture{}

func (b *BatchFuture) String() string        { return "<BatchFuture>" }
func (b *BatchFuture) Type() string          { return "BatchFuture" }
func (b *BatchFuture) Freeze()               {}
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
	results := make([]starlark.Value, len(b.futures))
	for i, future := range b.futures {
		var result starlark.Value
		if err := future.Get(&result); err != nil {
			return nil, err
		}
		results[i] = result
	}
	return starlark.NewList(results), nil
}

func (b *BatchFuture) isReadyMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	for _, future := range b.futures {
		if !future.IsReady() {
			return starlark.False, nil
		}
	}
	return starlark.True, nil
}

// Callable wraps a function and its arguments
type Callable struct {
	Fn   starlark.Value
	Args starlark.Tuple
}

var _ starlark.Value = &Callable{}

func (c *Callable) String() string        { return "<Callable>" }
func (c *Callable) Type() string          { return "Callable" }
func (c *Callable) Freeze()               {}
func (c *Callable) Truth() starlark.Bool  { return true }
func (c *Callable) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Callable") }
