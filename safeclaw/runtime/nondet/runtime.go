package nondet

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/mode"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/temporalops"
)

type TemporalConfig struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type TestClockConfig struct {
	// FixedUnixNano pins test-mode time to this epoch nanos value when provided.
	FixedUnixNano *int64
	// DisableAutoAdvanceOnSleep keeps sleep from advancing virtual clock.
	DisableAutoAdvanceOnSleep bool
}

type Runtime interface {
	Mode() mode.Value
	Execute(ctx context.Context, op string, payload []byte) ([]byte, error)
	Close() error
}

type runtime struct {
	mode mode.Value
	exec temporalops.Executor

	mu                 sync.Mutex
	baseUnixNano       int64
	offsetUnixNano     int64
	autoAdvanceOnSleep bool
}

func (r *runtime) Mode() mode.Value { return r.mode }
func (r *runtime) Execute(ctx context.Context, op string, payload []byte) ([]byte, error) {
	return r.exec.Execute(ctx, op, payload)
}
func (r *runtime) Close() error { return r.exec.Close() }

func New(modeValue mode.Value, temporal TemporalConfig, testClock TestClockConfig) Runtime {
	switch modeValue {
	case mode.Test:
		base := time.Now().UnixNano()
		if testClock.FixedUnixNano != nil {
			base = *testClock.FixedUnixNano
		}
		return &runtime{
			mode:               modeValue,
			exec:               temporalops.NewTestsuiteExecutor(),
			baseUnixNano:       base,
			autoAdvanceOnSleep: !testClock.DisableAutoAdvanceOnSleep,
		}
	case mode.Staging, mode.Prod:
		return &runtime{
			mode: modeValue,
			exec: temporalops.NewSDKExecutor(temporalops.SDKConfig{
				HostPort:  temporal.HostPort,
				Namespace: temporal.Namespace,
				TaskQueue: temporal.TaskQueue,
			}),
		}
	default:
		return &runtime{mode: mode.Dev, exec: &temporalops.LocalExecutor{}}
	}
}

func Now(ctx context.Context, r Runtime) (time.Time, error) {
	if rr, ok := r.(*runtime); ok && rr.mode == mode.Test {
		rr.mu.Lock()
		ts := rr.baseUnixNano + rr.offsetUnixNano
		rr.mu.Unlock()
		return time.Unix(0, ts), nil
	}
	b, err := r.Execute(ctx, temporalops.OpNow, nil)
	if err != nil {
		return time.Time{}, err
	}
	var out temporalops.NowOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, out.UnixNano), nil
}

func Sleep(ctx context.Context, r Runtime, d time.Duration) error {
	if rr, ok := r.(*runtime); ok && rr.mode == mode.Test {
		if d < 0 {
			d = 0
		}
		rr.mu.Lock()
		if rr.autoAdvanceOnSleep {
			rr.offsetUnixNano += int64(d)
		}
		rr.mu.Unlock()
		return nil
	}
	in, _ := json.Marshal(temporalops.SleepInput{DurationNanos: int64(d)})
	_, err := r.Execute(ctx, temporalops.OpSleep, in)
	return err
}

func RandInt(ctx context.Context, r Runtime, min, max int) (int, error) {
	in, _ := json.Marshal(temporalops.RandIntInput{Min: min, Max: max})
	b, err := r.Execute(ctx, temporalops.OpRandInt, in)
	if err != nil {
		return 0, err
	}
	var out temporalops.RandIntOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

func RandFloat(ctx context.Context, r Runtime) (float64, error) {
	b, err := r.Execute(ctx, temporalops.OpRandFloat, nil)
	if err != nil {
		return 0, err
	}
	var out temporalops.RandFloatOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, err
	}
	return out.Value, nil
}

func UUID4(ctx context.Context, r Runtime) (string, error) {
	b, err := r.Execute(ctx, temporalops.OpUUID4, nil)
	if err != nil {
		return "", err
	}
	var out temporalops.UUIDOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

func HTTPDo(ctx context.Context, r Runtime, in temporalops.HTTPRequestInput) ([]byte, error) {
	payload, _ := json.Marshal(in)
	return r.Execute(ctx, temporalops.OpHTTPRequestDo, payload)
}

func SQLiteExec(ctx context.Context, r Runtime, in temporalops.SQLiteExecInput) (temporalops.SQLiteExecOutput, error) {
	payload, _ := json.Marshal(in)
	raw, err := r.Execute(ctx, temporalops.OpSQLiteExec, payload)
	if err != nil {
		return temporalops.SQLiteExecOutput{}, err
	}
	var out temporalops.SQLiteExecOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return temporalops.SQLiteExecOutput{}, err
	}
	return out, nil
}

func SQLiteQuery(ctx context.Context, r Runtime, in temporalops.SQLiteQueryInput) (temporalops.SQLiteQueryOutput, error) {
	payload, _ := json.Marshal(in)
	raw, err := r.Execute(ctx, temporalops.OpSQLiteQuery, payload)
	if err != nil {
		return temporalops.SQLiteQueryOutput{}, err
	}
	var out temporalops.SQLiteQueryOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return temporalops.SQLiteQueryOutput{}, err
	}
	return out, nil
}

func MustGet(ctx context.Context, r Runtime) Runtime {
	if r == nil {
		return New(mode.Dev, TemporalConfig{}, TestClockConfig{})
	}
	return r
}

func EnsureNow(ctx context.Context, r Runtime, fallback time.Time) time.Time {
	v, err := Now(ctx, r)
	if err != nil {
		return fallback
	}
	return v
}

func Validate(r Runtime) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	return nil
}
