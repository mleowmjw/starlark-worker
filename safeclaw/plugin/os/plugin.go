package os

import (
	"context"
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "os"
}

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	environ := starlark.NewDict(len(info.Environ))
	for k, v := range info.Environ {
		environ.SetKey(starlark.String(k), starlark.String(v))
	}
	return &Module{environ: environ}
}

type Module struct {
	environ *starlark.Dict
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "os" }
func (f *Module) Type() string                          { return "os" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: os") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{}

var properties = map[string]star.PropertyFactory{
	"environ": environ,
}

func environ(receiver starlark.Value) (starlark.Value, error) {
	return receiver.(*Module).environ, nil
}
