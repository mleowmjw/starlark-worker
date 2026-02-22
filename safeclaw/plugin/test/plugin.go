package test

import (
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "test"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	return &Module{}
}

type Module struct{}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "test" }
func (f *Module) Type() string                          { return "test" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: test") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"true":      starlark.NewBuiltin("true", _true),
	"false":     starlark.NewBuiltin("false", _false),
	"equal":     starlark.NewBuiltin("equal", _equal),
	"not_equal": starlark.NewBuiltin("not_equal", _notEqual),
}

var properties = map[string]star.PropertyFactory{}

func _true(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	var message starlark.Value
	if err := starlark.UnpackArgs("true", args, kwargs, "v", &v, "message?", &message); err != nil {
		return nil, err
	}
	if !v.Truth() {
		if message != nil {
			return nil, fmt.Errorf("assertion failed: %s", message.String())
		}
		return nil, fmt.Errorf("assertion failed")
	}
	return starlark.None, nil
}

func _false(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	var message starlark.Value
	if err := starlark.UnpackArgs("false", args, kwargs, "v", &v, "message?", &message); err != nil {
		return nil, err
	}
	if v.Truth() {
		if message != nil {
			return nil, fmt.Errorf("assertion failed: %s", message.String())
		}
		return nil, fmt.Errorf("assertion failed")
	}
	return starlark.None, nil
}

func _equal(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expected, actual starlark.Value
	var message starlark.Value
	if err := starlark.UnpackArgs("equal", args, kwargs, "expected", &expected, "actual", &actual, "message?", &message); err != nil {
		return nil, err
	}
	if eq, err := starlark.Equal(expected, actual); err != nil {
		return nil, err
	} else if !eq {
		baseMsg := fmt.Sprintf("assertion failed\nExpected : %s\nActual   : %s", expected.String(), actual.String())
		if message != nil {
			return nil, fmt.Errorf("%s: %s", message.String(), baseMsg)
		}
		return nil, fmt.Errorf("%s", baseMsg)
	}
	return starlark.None, nil
}

func _notEqual(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expected, actual starlark.Value
	var message starlark.Value
	if err := starlark.UnpackArgs("not_equal", args, kwargs, "expected", &expected, "actual", &actual, "message?", &message); err != nil {
		return nil, err
	}
	if eq, err := starlark.Equal(expected, actual); err != nil {
		return nil, err
	} else if eq {
		baseMsg := fmt.Sprintf("assertion failed: values should not be equal\nValue: %s", expected.String())
		if message != nil {
			return nil, fmt.Errorf("%s: %s", message.String(), baseMsg)
		}
		return nil, fmt.Errorf("%s", baseMsg)
	}
	return starlark.None, nil
}
