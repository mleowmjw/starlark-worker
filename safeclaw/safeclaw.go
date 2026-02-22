package safeclaw

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
	temp "go.temporal.io/sdk/workflow"
)

// Plugin is the safeclaw plugin contract.
// Plugins provide Starlark modules that can be loaded via load("@plugin", "pluginID").
type Plugin interface {
	// ID returns the unique plugin identifier (e.g., "json", "time", "request").
	ID() string
	
	// Module creates a new Starlark module instance for this plugin.
	// The module is created per-execution and receives context and runtime info.
	// The ctx parameter may be either context.Context or workflow.Context (from Temporal).
	Module(ctx any, info RunInfo) starlark.Value
}

// Registrar is an optional interface that plugins can implement to register
// their Temporal activities with a worker.
type Registrar interface {
	// RegisterActivities registers all activities used by this plugin.
	// The registerFn is typically worker.RegisterActivity from a Temporal worker.
	RegisterActivities(registerFn func(activity any))
}

// RunInfo provides contextual information about the current script execution.
type RunInfo struct {
	// Environ contains environment variables available to the script.
	Environ map[string]string

	// StartTime is when the script execution began.
	StartTime time.Time

	// Logger is the structured logger for this execution.
	Logger *slog.Logger
}

// Runner executes Starlark scripts with a set of registered plugins.
type Runner struct {
	plugins  map[string]Plugin
	logger   *slog.Logger
	builtins starlark.StringDict
}

// NewRunner creates a new Starlark script runner with the given plugins.
// If logger is nil, a default logger writing to stderr is used.
func NewRunner(plugins []Plugin, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	
	pluginMap := make(map[string]Plugin, len(plugins))
	for _, p := range plugins {
		pluginMap[p.ID()] = p
	}
	
	// Builtins available to all scripts
	builtins := starlark.StringDict{
		"CallableObject": star.CallableObjectConstructor,
		"Dataclass":      star.DataclassConstructor,
	}
	
	return &Runner{
		plugins:  pluginMap,
		logger:   logger,
		builtins: builtins,
	}
}

// RegisterActivities registers all activities from plugins that implement the Registrar interface.
// The registerFn is typically worker.RegisterActivity from a Temporal worker.
// This should be called during worker setup before starting the worker.
func (r *Runner) RegisterActivities(registerFn func(activity any)) {
	for _, plugin := range r.plugins {
		if registrar, ok := plugin.(Registrar); ok {
			registrar.RegisterActivities(registerFn)
		}
	}
}

// RunScript executes a function from a Starlark script in the given filesystem.
// The ctx parameter may be either context.Context or workflow.Context (from Temporal).
func (r *Runner) RunScript(ctx any, fs star.FS, path, function string, args ...any) (starlark.Value, error) {
	return r.run(ctx, fs, path, function, args...)
}

// RunTar executes a function from a Starlark script stored in a gzipped tar archive.
// The ctx parameter may be either context.Context or workflow.Context (from Temporal).
func (r *Runner) RunTar(ctx any, tarData []byte, path, function string, args ...any) (starlark.Value, error) {
	fs, err := star.NewTarFS(tarData)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar filesystem: %w", err)
	}
	return r.run(ctx, fs, path, function, args...)
}

// RunSource executes a function from inline Starlark source code.
// The source is treated as a file named "main.star".
// The ctx parameter may be either context.Context or workflow.Context (from Temporal).
func (r *Runner) RunSource(ctx any, source []byte, function string, args ...any) (starlark.Value, error) {
	fs := star.NewMemoryFS(map[string][]byte{
		"main.star": source,
	})
	return r.run(ctx, fs, "main.star", function, args...)
}

// run is the internal execution method.
func (r *Runner) run(ctx any, fs star.FS, path, function string, args ...any) (result starlark.Value, err error) {
	startTime := time.Now()

	// Create execution context
	info := RunInfo{
		Environ:   make(map[string]string),
		StartTime: startTime,
		Logger:    r.logger,
	}

	// Setup workflow backend based on environment
	backend, workflowCtx := r.setupWorkflowBackend(ctx, info)
	
	// Wrap logger with replay-aware handler to suppress logs during replay
	replayAwareLogger := slog.New(workflow.NewReplayAwareHandler(
		r.logger.Handler(),
		func() bool { return backend.IsReplaying() },
	))
	
	// Update info with replay-aware logger
	info.Logger = replayAwareLogger

	// Initialize plugin modules with the appropriate context
	pluginModules := starlark.StringDict{}
	for id, plugin := range r.plugins {
		pluginModules[id] = plugin.Module(workflowCtx, info)
	}

	// Create Starlark thread
	thread := &starlark.Thread{
		Name: "safeclaw",
		Print: func(_ *starlark.Thread, msg string) {
			replayAwareLogger.Info(msg)
		},
	}

	// Store context and backend in thread-local storage for plugins to access
	// Try to extract standard context for thread local storage
	var stdCtx context.Context
	if c, ok := ctx.(context.Context); ok {
		stdCtx = c
	} else if c, ok := workflowCtx.(context.Context); ok {
		stdCtx = c
	} else {
		stdCtx = context.Background()
	}

	thread.SetLocal("ctx", stdCtx)
	thread.SetLocal("workflow_ctx", workflowCtx)
	thread.SetLocal("workflow_backend", backend)
	thread.SetLocal("logger", replayAwareLogger)

	// Fix Bug 4: Store atexit module in thread-local storage for register/unregister functions
	if atexitModule, ok := pluginModules["atexit"]; ok {
		thread.SetLocal("atexit_module", atexitModule)
	}

	// Setup module loader
	thread.Load = star.ThreadLoad(fs, r.builtins, map[string]starlark.StringDict{
		"plugin": pluginModules,
	})

	// Convert Go args to Starlark args
	starArgs := make(starlark.Tuple, len(args))
	for i, arg := range args {
		v, err := star.ToStarlark(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to convert arg %d: %w", i, err)
		}
		starArgs[i] = v
	}

	// Execute the function
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic during execution: %v", rec)
		}
	}()

	result, err = star.Call(thread, path, function, starArgs, nil)
	if err != nil {
		var evalErr *starlark.EvalError
		if errors.As(err, &evalErr) {
			r.logger.Error("starlark execution error",
				"backtrace", evalErr.Backtrace(),
				"error", err.Error())
		}
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return result, nil
}

// GetContext retrieves the context.Context from a Starlark thread.
// This is a helper for plugins to access the execution context.
func GetContext(t *starlark.Thread) context.Context {
	ctx, ok := t.Local("ctx").(context.Context)
	if !ok {
		return context.Background()
	}
	return ctx
}

// GetLogger retrieves the slog.Logger from a Starlark thread.
// This is a helper for plugins to access the logger.
func GetLogger(t *starlark.Thread) *slog.Logger {
	logger, ok := t.Local("logger").(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}

// setupWorkflowBackend sets up the workflow backend based on the environment.
// Returns the backend and a context that plugins can use (may be temp.Context or context.Context).
func (r *Runner) setupWorkflowBackend(ctx any, info RunInfo) (workflow.Backend, any) {
	mode := workflow.GetMode(info.Environ)

	// Check if we're already in a Temporal workflow context
	if tempCtx, ok := ctx.(temp.Context); ok {
		// We're in a Temporal workflow, use Temporal backend regardless of mode
		backend := workflow.NewTemporalBackend(tempCtx)
		// Store the backend in the Temporal context
		enrichedCtx := temp.WithValue(tempCtx, "safeclaw.workflow.backend", backend)
		return backend, enrichedCtx
	}

	// Extract standard context
	var stdCtx context.Context
	if c, ok := ctx.(context.Context); ok {
		stdCtx = c
	} else {
		stdCtx = context.Background()
	}

	// Not in Temporal workflow context, use local backend
	backend := workflow.NewLocalBackend(stdCtx)
	// Store the backend in the standard context
	enrichedCtx := context.WithValue(stdCtx, "safeclaw.workflow.backend", backend)

	if mode == workflow.ModeWorkflow {
		// Workflow mode requested but not in a workflow context
		r.logger.Warn("workflow mode requested but not in workflow context, using local backend")
	}

	return backend, enrichedCtx
}
