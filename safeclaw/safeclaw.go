package safeclaw

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/mode"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/nondet"
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

	// Mode controls runtime behavior of nondeterministic plugins.
	Mode mode.Value
	// Runtime executes nondeterministic operations according to the selected mode.
	Runtime nondet.Runtime

	// StartTime is when the script execution began.
	StartTime time.Time

	// Logger is the structured logger for this execution.
	Logger *slog.Logger
}

type TemporalRuntimeOptions struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type TestRuntimeOptions struct {
	// FixedUnixNano pins test-mode clock to this time.
	FixedUnixNano *int64
	// DisableAutoAdvanceOnSleep keeps sleep from changing virtual test clock.
	DisableAutoAdvanceOnSleep bool
}

// RunOptions configures one script execution.
type RunOptions struct {
	// Environ contains per-run environment variables passed to plugins.
	Environ map[string]string
	// Temporal contains optional Temporal runtime settings for non-dev modes.
	Temporal TemporalRuntimeOptions
	// Test configures deterministic test-mode clock behavior.
	Test TestRuntimeOptions
}

const threadLocalRuntimeKey = "nondet_runtime"

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

// RunScript executes a function from a Starlark script in the given filesystem.
func (r *Runner) RunScript(ctx context.Context, fs star.FS, path, function string, args ...any) (starlark.Value, error) {
	return r.run(ctx, fs, path, function, args, RunOptions{})
}

// RunScriptWithOptions executes a function from a Starlark script with per-run options.
func (r *Runner) RunScriptWithOptions(ctx context.Context, fs star.FS, path, function string, opts RunOptions, args ...any) (starlark.Value, error) {
	return r.run(ctx, fs, path, function, args, opts)
}

// RunTar executes a function from a Starlark script stored in a gzipped tar archive.
func (r *Runner) RunTar(ctx context.Context, tarData []byte, path, function string, args ...any) (starlark.Value, error) {
	fs, err := star.NewTarFS(tarData)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar filesystem: %w", err)
	}
	return r.run(ctx, fs, path, function, args, RunOptions{})
}

// RunTarWithOptions executes a function from a gzipped tar script with per-run options.
func (r *Runner) RunTarWithOptions(ctx context.Context, tarData []byte, path, function string, opts RunOptions, args ...any) (starlark.Value, error) {
	fs, err := star.NewTarFS(tarData)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar filesystem: %w", err)
	}
	return r.run(ctx, fs, path, function, args, opts)
}

// RunSource executes a function from inline Starlark source code.
// The source is treated as a file named "main.star".
func (r *Runner) RunSource(ctx context.Context, source []byte, function string, args ...any) (starlark.Value, error) {
	fs := star.NewMemoryFS(map[string][]byte{
		"main.star": source,
	})
	return r.run(ctx, fs, "main.star", function, args, RunOptions{})
}

// RunSourceWithOptions executes source code with per-run options.
func (r *Runner) RunSourceWithOptions(ctx context.Context, source []byte, function string, opts RunOptions, args ...any) (starlark.Value, error) {
	fs := star.NewMemoryFS(map[string][]byte{
		"main.star": source,
	})
	return r.run(ctx, fs, "main.star", function, args, opts)
}

// run is the internal execution method.
func (r *Runner) run(ctx context.Context, fs star.FS, path, function string, args []any, opts RunOptions) (result starlark.Value, err error) {
	environ := mergeEnviron(opts.Environ)
	resolvedMode := resolveMode(environ)
	rt := nondet.New(resolvedMode, nondet.TemporalConfig{
		HostPort:  opts.Temporal.HostPort,
		Namespace: opts.Temporal.Namespace,
		TaskQueue: opts.Temporal.TaskQueue,
	}, nondet.TestClockConfig{
		FixedUnixNano:             opts.Test.FixedUnixNano,
		DisableAutoAdvanceOnSleep: opts.Test.DisableAutoAdvanceOnSleep,
	})
	defer func() {
		if closeErr := rt.Close(); closeErr != nil && r.logger != nil {
			r.logger.Warn("nondet runtime close failed", "error", closeErr)
		}
	}()
	startTime := nondet.EnsureNow(ctx, rt, time.Now())

	// Create execution context
	info := RunInfo{
		Environ:   environ,
		Mode:      resolvedMode,
		Runtime:   rt,
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
	thread.SetLocal(threadLocalRuntimeKey, rt)

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

func mergeEnviron(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)
	return dst
}

func resolveMode(environ map[string]string) mode.Value {
	modeEnv := map[string]string{
		mode.EnvChameleonMode: stringsOrDefault(environ[mode.EnvChameleonMode], os.Getenv(mode.EnvChameleonMode)),
		mode.EnvSafeclawEnv:   stringsOrDefault(environ[mode.EnvSafeclawEnv], os.Getenv(mode.EnvSafeclawEnv)),
	}
	return mode.Resolve(modeEnv)
}

func stringsOrDefault(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
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

// GetRuntime retrieves the nondeterministic runtime from a Starlark thread.
func GetRuntime(t *starlark.Thread) nondet.Runtime {
	if rt, ok := t.Local(threadLocalRuntimeKey).(nondet.Runtime); ok && rt != nil {
		return rt
	}
	return nondet.New(mode.Dev, nondet.TemporalConfig{}, nondet.TestClockConfig{})
}
