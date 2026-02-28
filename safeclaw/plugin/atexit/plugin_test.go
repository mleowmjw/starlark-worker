package atexit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestRegisterUnregisterWithoutThreadLocalModule(t *testing.T) {
	module := &Module{hooks: &ExitHooks{}}
	thread := &starlark.Thread{Name: "test-atexit"}
	fn := starlark.NewBuiltin("dummy", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	registerValue, err := module.Attr("register")
	require.NoError(t, err)
	registerBuiltin, ok := registerValue.(*starlark.Builtin)
	require.True(t, ok)

	_, err = starlark.Call(thread, registerBuiltin, starlark.Tuple{fn}, nil)
	require.NoError(t, err)
	require.Len(t, module.hooks.hooks, 1)

	unregisterValue, err := module.Attr("unregister")
	require.NoError(t, err)
	unregisterBuiltin, ok := unregisterValue.(*starlark.Builtin)
	require.True(t, ok)

	_, err = starlark.Call(thread, unregisterBuiltin, starlark.Tuple{fn}, nil)
	require.NoError(t, err)
	require.Len(t, module.hooks.hooks, 0)
}

func TestExitHooksRunOrderLIFO(t *testing.T) {
	module := &Module{hooks: &ExitHooks{}}
	thread := &starlark.Thread{Name: "test-atexit-order"}
	order := make([]string, 0, 2)
	fnA := starlark.NewBuiltin("a", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		order = append(order, "A")
		return starlark.None, nil
	})
	fnB := starlark.NewBuiltin("b", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		order = append(order, "B")
		return starlark.None, nil
	})

	registerValue, err := module.Attr("register")
	require.NoError(t, err)
	registerBuiltin := registerValue.(*starlark.Builtin)

	_, err = starlark.Call(thread, registerBuiltin, starlark.Tuple{fnA}, nil)
	require.NoError(t, err)
	_, err = starlark.Call(thread, registerBuiltin, starlark.Tuple{fnB}, nil)
	require.NoError(t, err)

	err = module.hooks.Run(thread)
	require.NoError(t, err)
	require.Equal(t, []string{"B", "A"}, order)
}

func TestExitHooksRunAggregatesErrors(t *testing.T) {
	hooks := &ExitHooks{}
	okFn := starlark.NewBuiltin("ok", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
	errFn := starlark.NewBuiltin("err", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, fmt.Errorf("boom")
	})

	hooks.Register(okFn, nil, nil)
	hooks.Register(errFn, nil, nil)

	err := hooks.Run(&starlark.Thread{Name: "test-atexit-errors"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}
