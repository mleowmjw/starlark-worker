package progress

import (
	"fmt"
	"log/slog"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "progress"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	return &Module{
		logger: info.Logger,
	}
}

type Module struct {
	logger *slog.Logger
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "progress" }
func (f *Module) Type() string                          { return "progress" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: progress") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

const (
	TaskStatePending   = "PENDING"
	TaskStateRunning   = "RUNNING"
	TaskStateSucceeded = "SUCCEEDED"
	TaskStateFailed    = "FAILED"
	TaskStateKilled    = "KILLED"
	TaskStateSkipped   = "SKIPPED"
)

var builtins = map[string]*starlark.Builtin{
	"report": starlark.NewBuiltin("report", report),
}

var properties = map[string]star.PropertyFactory{
	"task_state_running":   func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStateRunning), nil },
	"task_state_pending":   func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStatePending), nil },
	"task_state_succeeded": func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStateSucceeded), nil },
	"task_state_failed":    func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStateFailed), nil },
	"task_state_killed":    func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStateKilled), nil },
	"task_state_skipped":   func(receiver starlark.Value) (starlark.Value, error) { return starlark.String(TaskStateSkipped), nil },
}

func report(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// report(progress: str)
	// Report a progress string

	logger := safeclaw.GetLogger(t)

	var progressStr starlark.String

	if err := starlark.UnpackArgs("report", args, kwargs, "progress", &progressStr); err != nil {
		logger.Error("progress.report: unpack args failed", "error", err)
		return nil, err
	}

	logger.Info("progress", "msg", string(progressStr))
	return starlark.None, nil
}
