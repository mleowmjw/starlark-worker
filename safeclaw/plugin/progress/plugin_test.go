package progress

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

type capturedRecord struct {
	Message string
	Attrs   map[string]any
}

type captureHandler struct {
	mu      sync.Mutex
	records []capturedRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]any{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	h.records = append(h.records, capturedRecord{Message: r.Message, Attrs: attrs})
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureHandler) snapshot() []capturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]capturedRecord, len(h.records))
	copy(out, h.records)
	return out
}

func TestReportLogsWithoutThreadLocalLogger(t *testing.T) {
	handler := &captureHandler{}
	module := &Module{logger: slog.New(handler)}
	thread := &starlark.Thread{Name: "progress-no-thread-local"}

	v, err := module.Attr("report")
	require.NoError(t, err)
	builtin, ok := v.(*starlark.Builtin)
	require.True(t, ok)

	_, err = starlark.Call(thread, builtin, starlark.Tuple{starlark.String("Starting task")}, nil)
	require.NoError(t, err)

	records := handler.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, "progress", records[0].Message)
	require.Equal(t, "Starting task", records[0].Attrs["msg"])
}

func TestReportFallsBackToThreadLocalLogger(t *testing.T) {
	handler := &captureHandler{}
	module := &Module{logger: nil}
	thread := &starlark.Thread{Name: "progress-thread-local-fallback"}
	thread.SetLocal("logger", slog.New(handler))

	v, err := module.Attr("report")
	require.NoError(t, err)
	builtin := v.(*starlark.Builtin)

	_, err = starlark.Call(thread, builtin, starlark.Tuple{starlark.String("Task complete")}, nil)
	require.NoError(t, err)

	records := handler.snapshot()
	require.Len(t, records, 1)
	require.Equal(t, "progress", records[0].Message)
	require.Equal(t, "Task complete", records[0].Attrs["msg"])
}

func TestReportBadArgsReturnsError(t *testing.T) {
	handler := &captureHandler{}
	module := &Module{logger: slog.New(handler)}
	thread := &starlark.Thread{Name: "progress-bad-args"}

	v, err := module.Attr("report")
	require.NoError(t, err)
	builtin := v.(*starlark.Builtin)

	_, err = starlark.Call(thread, builtin, nil, nil)
	require.Error(t, err)
}

func TestTaskStateConstantsStable(t *testing.T) {
	module := &Module{}
	cases := map[string]string{
		"task_state_pending":   TaskStatePending,
		"task_state_running":   TaskStateRunning,
		"task_state_succeeded": TaskStateSucceeded,
		"task_state_failed":    TaskStateFailed,
		"task_state_killed":    TaskStateKilled,
		"task_state_skipped":   TaskStateSkipped,
	}
	for attr, expected := range cases {
		v, err := module.Attr(attr)
		require.NoError(t, err)
		s, ok := v.(starlark.String)
		require.True(t, ok)
		require.Equal(t, expected, s.GoString())
	}
}
