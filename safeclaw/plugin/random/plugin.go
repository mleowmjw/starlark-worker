package random

import (
	"context"
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/ext"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/mode"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/nondet"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "random"
}

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	m := &Module{
		runtime: info.Runtime,
	}
	m.attributes = map[string]starlark.Value{
		"seed":    starlark.NewBuiltin("seed", m.seedFn).BindReceiver(m),
		"randint": starlark.NewBuiltin("randint", m.randIntFn).BindReceiver(m),
		"random":  starlark.NewBuiltin("random", m.randFn).BindReceiver(m),
	}
	return m
}

type Module struct {
	attributes map[string]starlark.Value
	seed       *int64
	counter    uint64
	runtime    nondet.Runtime
}

var _ starlark.HasAttrs = &Module{}

func (m *Module) String() string                        { return "random" }
func (m *Module) Type() string                          { return "random" }
func (m *Module) Freeze()                               {}
func (m *Module) Truth() starlark.Bool                  { return true }
func (m *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: random") }
func (m *Module) Attr(n string) (starlark.Value, error) { return m.attributes[n], nil }
func (m *Module) AttrNames() []string                   { return ext.SortedKeys(m.attributes) }

// seedFn is used to seed the random number generator
func (m *Module) seedFn(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var seed int64
	if err := starlark.UnpackArgs("seed", args, kwargs, "seed", &seed); err != nil {
		logger.Error("random.seed: unpack args failed", "error", err)
		return nil, err
	}

	m.seed = &seed
	m.counter = 0

	return starlark.None, nil
}

// randIntFn generates a random integer between the specified range
func (m *Module) randIntFn(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var min, max int
	if err := starlark.UnpackArgs("randint", args, kwargs, "min", &min, "max", &max); err != nil {
		logger.Error("random.randint: unpack args failed", "error", err)
		return nil, err
	}

	var v int
	if m.seed != nil && m.runtime.Mode() != mode.Dev {
		m.counter++
		var err error
		v, err = nondet.RandIntSeeded(safeclaw.GetContext(t), m.runtime, min, max, *m.seed, m.counter)
		if err != nil {
			logger.Error("random.randint: seeded nondet runtime failed", "error", err)
			return nil, err
		}
	} else if m.seed != nil {
		v, _ = nondet.RandIntSeeded(safeclaw.GetContext(t), m.runtime, min, max, *m.seed, m.counter+1)
		m.counter++
	} else {
		var err error
		v, err = nondet.RandInt(safeclaw.GetContext(t), m.runtime, min, max)
		if err != nil {
			logger.Error("random.randint: nondet runtime failed", "error", err)
			return nil, err
		}
	}

	return starlark.MakeInt(v), nil
}

// randFn generates a random floating point number between 0 and 1
func (m *Module) randFn(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v float64
	var err error
	if m.seed != nil {
		m.counter++
		v, err = nondet.RandFloatSeeded(safeclaw.GetContext(t), m.runtime, *m.seed, m.counter)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		v, err = nondet.RandFloat(safeclaw.GetContext(t), m.runtime)
		if err != nil {
			return nil, err
		}
	}

	return starlark.Float(v), nil
}
