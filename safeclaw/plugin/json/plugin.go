package json

import (
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "json"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	return &Module{}
}

type Module struct{}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "json" }
func (f *Module) Type() string                          { return "json" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: json") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"dumps": starlark.NewBuiltin("dumps", dumps),
	"loads": starlark.NewBuiltin("loads", loads),
}

var properties = map[string]star.PropertyFactory{}

func dumps(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// dumps(obj)
	// Serialize `obj` to a JSON formatted `str`

	logger := safeclaw.GetLogger(t)

	var obj starlark.Value

	if err := starlark.UnpackArgs("dumps", args, kwargs, "obj", &obj); err != nil {
		logger.Error("json.dumps: unpack args failed", "error", err)
		return nil, err
	}

	encoded, err := star.Encode(obj)
	if err != nil {
		logger.Error("json.dumps: encode failed", "error", err)
		return nil, err
	}
	return starlark.String(encoded), nil
}

func loads(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Deserialize `s` (a `string` or `bytes` instance containing a JSON document)
	// to a Python object.

	logger := safeclaw.GetLogger(t)

	var s starlark.Value

	if err := starlark.UnpackArgs("loads", args, kwargs, "s", &s); err != nil {
		logger.Error("json.loads: unpack args failed", "error", err)
		return nil, err
	}

	var sb []byte
	switch s := s.(type) {
	case starlark.String:
		sb = []byte(s)
	case starlark.Bytes:
		sb = []byte(s)
	default:
		err := fmt.Errorf("argument must be a string or bytes; actual: %T: %s", s, s.String())
		logger.Error("json.loads: invalid argument type", "error", err)
		return nil, err
	}

	var res starlark.Value
	if err := star.Decode(sb, &res); err != nil {
		logger.Error("json.loads: decode failed", "error", err)
		return nil, err
	}
	return res, nil
}
