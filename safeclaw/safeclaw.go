package safeclaw

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

// Plugin is the safeclaw plugin contract.
// Plugins provide Starlark modules that can be loaded via load("@plugin", "pluginID").
type Plugin interface {
	// ID returns the unique plugin identifier (e.g., "json", "time", "request").
	ID() string
	
	// Module creates a new Starlark module instance for this plugin.
	// The module is created per-execution and receives context and runtime info.
	Module(ctx context.Context, info RunInfo) starlark.Value
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
	plugins map[string]Plugin
	logger  *slog.Logger
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

// RunScript executes a function from a Starlark script in the given filesystem.
func (r *Runner) RunScript(ctx context.Context, fs star.FS, path, function string, args ...any) (starlark.Value, error) {
	return r.run(ctx, fs, path, function, args...)
}

// RunTar executes a function from a Starlark script stored in a gzipped tar archive.
func (r *Runner) RunTar(ctx context.Context, tarData []byte, path, function string, args ...any) (starlark.Value, error) {
	fs, err := star.NewTarFS(tarData)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar filesystem: %w", err)
	}
	return r.run(ctx, fs, path, function, args...)
}

// RunSource executes a function from inline Starlark source code.
// The source is treated as a file named "main.star".
func (r *Runner) RunSource(ctx context.Context, source []byte, function string, args ...any) (starlark.Value, error) {
	fs := star.NewMemoryFS(map[string][]byte{
		"main.star": source,
	})
	return r.run(ctx, fs, "main.star", function, args...)
}

// run is the internal execution method.
func (r *Runner) run(ctx context.Context, fs star.FS, path, function string, args ...any) (result starlark.Value, err error) {
	startTime := time.Now()
	
	// Create execution context
	info := RunInfo{
		Environ:   make(map[string]string),
		StartTime: startTime,
		Logger:    r.logger,
	}
	
	// Initialize plugin modules
	pluginModules := starlark.StringDict{}
	for id, plugin := range r.plugins {
		pluginModules[id] = plugin.Module(ctx, info)
	}
	
	// Create Starlark thread
	thread := &starlark.Thread{
		Name: "safeclaw",
		Print: func(_ *starlark.Thread, msg string) {
			r.logger.Info(msg)
		},
	}
	
	// Store context in thread-local storage for plugins to access
	thread.SetLocal("ctx", ctx)
	thread.SetLocal("logger", r.logger)
	
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
