package time

import (
	"context"

	"fmt"
	"strings"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/ext"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "time"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	// Calculate delta for effective time (for backfill scenarios)
	effectiveTime := info.StartTime
	if timeStr, ok := info.Environ["STARLARK_TIME"]; ok {
		if t, err := parseStarlarkTime(timeStr); err == nil {
			effectiveTime = t
		}
	}
	delta := effectiveTime.Sub(info.StartTime)

	// Get workflow backend from context
	backend := workflow.GetBackend(ctx)

	// Extract the standard context if possible
	var stdCtx context.Context
	if c, ok := ctx.(context.Context); ok {
		stdCtx = c
	} else {
		stdCtx = context.Background()
	}

	m := &Module{
		delta:   delta,
		backend: backend,
		ctx:     stdCtx,
	}
	m.attributes = map[string]starlark.Value{
		"sleep":              starlark.NewBuiltin("sleep", _sleep).BindReceiver(m),
		"time_ns":            starlark.NewBuiltin("time_ns", _time_ns).BindReceiver(m),
		"time":               starlark.NewBuiltin("time", _time).BindReceiver(m),
		"utc_format_seconds": starlark.NewBuiltin("utc_format_seconds", _utc_format_seconds).BindReceiver(m),
	}
	return m
}

type Module struct {
	attributes map[string]starlark.Value
	delta      time.Duration
	backend    workflow.Backend
	ctx        context.Context
}

func (m *Module) String() string                        { return "time" }
func (m *Module) Type() string                          { return "time" }
func (m *Module) Freeze()                               {}
func (m *Module) Truth() starlark.Bool                  { return true }
func (m *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: time") }
func (m *Module) Attr(n string) (starlark.Value, error) { return m.attributes[n], nil }
func (m *Module) AttrNames() []string                   { return ext.SortedKeys(m.attributes) }

var _ starlark.HasAttrs = &Module{}

// _sleep suspends execution of the calling thread for the given number of seconds.
func _sleep(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)
	receiver := fn.Receiver().(*Module)

	var seconds starlark.Value
	if err := starlark.UnpackArgs("sleep", args, kwargs, "seconds", &seconds); err != nil {
		logger.Error("time.sleep: unpack args failed", "error", err)
		return nil, err
	}

	var sf float64
	switch arg0 := seconds.(type) {
	case starlark.Int:
		sf = float64(arg0.Float())
	case starlark.Float:
		sf = float64(arg0)
	default:
		err := fmt.Errorf("bad argument type: %T: %v", seconds, seconds)
		logger.Error("time.sleep: invalid argument", "error", err)
		return starlark.None, err
	}

	duration := time.Duration(float64(time.Second) * sf)

	// Use workflow backend if available, otherwise direct execution
	if receiver.backend != nil && receiver.backend.InWorkflow() {
		if err := receiver.backend.Sleep(duration); err != nil {
			logger.Error("time.sleep: workflow sleep failed", "error", err)
			return nil, err
		}
		return starlark.None, nil
	}

	// Direct execution (dev mode) - respect context cancellation
	ctx := receiver.ctx
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return starlark.None, nil
	case <-ctx.Done():
		logger.Info("time.sleep: interrupted by context cancellation", "elapsed", duration)
		return nil, ctx.Err()
	}
}

// _time_ns returns time as an integer number of nanoseconds since the epoch.
func _time_ns(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	receiver := fn.Receiver().(*Module)

	var ns int64
	if receiver.backend != nil && receiver.backend.InWorkflow() {
		// Use workflow-backed time (deterministic)
		receiver.backend.SideEffect(func() any {
			return receiver.backend.Now().Add(receiver.delta).UnixNano()
		}).Get(&ns)
	} else {
		// Direct execution (dev mode)
		ns = time.Now().Add(receiver.delta).UnixNano()
	}

	return starlark.MakeInt64(ns), nil
}

// _time returns the current unix time in seconds as floating point number.
func _time(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	receiver := fn.Receiver().(*Module)

	var ns int64
	if receiver.backend != nil && receiver.backend.InWorkflow() {
		// Use workflow-backed time (deterministic)
		receiver.backend.SideEffect(func() any {
			return receiver.backend.Now().Add(receiver.delta).UnixNano()
		}).Get(&ns)
	} else {
		// Direct execution (dev mode)
		ns = time.Now().Add(receiver.delta).UnixNano()
	}

	sec := float64(ns) / 1e9
	return starlark.Float(sec), nil
}

// _utc_format_seconds converts the given unix time in seconds to a string as specified by the format argument.
func _utc_format_seconds(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var format string
	var seconds float64
	if err := starlark.UnpackArgs("format_time", args, kw,
		"format", &format,
		"seconds", &seconds,
	); err != nil {
		logger.Error("time.utc_format_seconds: unpack args failed", "error", err)
		return nil, err
	}

	replacer := strings.NewReplacer("%Y", "2006", "%m", "01", "%d", "02", "%H", "15", "%M", "04", "%S", "05", "%y", "06")
	format = replacer.Replace(format)
	if strings.Contains(format, "%") {
		return nil, fmt.Errorf("unsupported date format: %s", format)
	}

	res := time.Unix(int64(seconds), 0).UTC().Format(format)
	return starlark.String(res), nil
}

// parseStarlarkTime parses STARLARK_TIME environment variable.
// Format: "unix:<seconds>"
func parseStarlarkTime(value string) (time.Time, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid STARLARK_TIME format: %s", value)
	}

	scheme := parts[0]
	rest := parts[1]

	if scheme != "unix" {
		return time.Time{}, fmt.Errorf("unsupported STARLARK_TIME scheme: %s", scheme)
	}

	var seconds int64
	if _, err := fmt.Sscanf(rest, "%d", &seconds); err != nil {
		return time.Time{}, fmt.Errorf("invalid STARLARK_TIME value: %s", rest)
	}

	return time.Unix(seconds, 0), nil
}
