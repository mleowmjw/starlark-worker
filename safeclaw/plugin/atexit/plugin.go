package atexit

import (
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "atexit"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	m := &Module{
		hooks: &ExitHooks{},
	}
	m.builtins = map[string]*starlark.Builtin{
		"register":   starlark.NewBuiltin("register", register).BindReceiver(m),
		"unregister": starlark.NewBuiltin("unregister", unregister).BindReceiver(m),
	}
	return m
}

type Module struct {
	hooks    *ExitHooks
	builtins map[string]*starlark.Builtin
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "atexit" }
func (f *Module) Type() string                          { return "atexit" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: atexit") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, f.builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(f.builtins, properties) }

var properties = map[string]star.PropertyFactory{}

func register(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	module := fn.Receiver().(*Module)

	callable := args[0].(starlark.Callable)
	args = args[1:]
	module.hooks.Register(callable, args, kwargs)
	return starlark.None, nil
}

func unregister(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	module := fn.Receiver().(*Module)

	callable := args[0].(starlark.Callable)
	module.hooks.Unregister(callable)
	return starlark.None, nil
}

// ExitHooks manages exit hooks that run when the script completes.
type ExitHooks struct {
	hooks []*exitHook
}

type exitHook struct {
	fn     starlark.Callable
	args   starlark.Tuple
	kwargs []starlark.Tuple
}

func (r *ExitHooks) Register(fn starlark.Callable, args starlark.Tuple, kwargs []starlark.Tuple) {
	if fn == nil {
		return
	}
	h := &exitHook{fn: fn, args: args, kwargs: kwargs}
	r.hooks = append(r.hooks, h)
}

func (r *ExitHooks) Unregister(fn starlark.Callable) {
	for i, h := range r.hooks {
		if h.fn == fn {
			r.hooks = append(r.hooks[:i], r.hooks[i+1:]...)
			break
		}
	}
}

func (r *ExitHooks) Run(t *starlark.Thread) error {
	hooks := r.hooks
	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		if _, err := starlark.Call(t, h.fn, h.args, h.kwargs); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("exit hooks failed: %v", errs)
	}
	return nil
}
