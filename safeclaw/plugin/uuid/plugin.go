package uuid

import (
	"fmt"
	"strings"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"github.com/google/uuid"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "uuid"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	backend := workflow.GetBackend(ctx)
	return &Module{
		backend: backend,
	}
}

type Module struct {
	backend workflow.Backend
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string        { return "uuid" }
func (f *Module) Type() string          { return "uuid" }
func (f *Module) Freeze()               {}
func (f *Module) Truth() starlark.Bool  { return true }
func (f *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: uuid") }
func (m *Module) Attr(n string) (starlark.Value, error) {
	if builtin, ok := m.builtins()[n]; ok {
		return builtin, nil
	}
	return star.Attr(m, n, nil, properties)
}

func (m *Module) AttrNames() []string {
	names := []string{}
	for name := range m.builtins() {
		names = append(names, name)
	}
	return names
}

func (m *Module) builtins() map[string]*starlark.Builtin {
	return map[string]*starlark.Builtin{
		"uuid4": starlark.NewBuiltin("uuid4", m.uuid4).BindReceiver(m),
	}
}

var properties = map[string]star.PropertyFactory{}

func (m *Module) uuid4(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	if err := starlark.UnpackArgs("uuid4", args, kwargs); err != nil {
		logger.Error("uuid.uuid4: unpack args failed", "error", err)
		return nil, err
	}

	var stringUUID string
	m.backend.SideEffect(func() any {
		return uuid.New().String()
	}).Get(&stringUUID)

	return &UUID{StringUUID: starlark.String(stringUUID)}, nil
}

// UUID is a Starlark value representing a UUID
type UUID struct {
	StringUUID starlark.String
}

var _ starlark.Value = &UUID{}
var _ starlark.HasAttrs = &UUID{}

func (u *UUID) String() string        { return string(u.StringUUID) }
func (u *UUID) Type() string          { return "uuid" }
func (u *UUID) Freeze()               {}
func (u *UUID) Truth() starlark.Bool  { return true }
func (u *UUID) Hash() (uint32, error) { return u.StringUUID.Hash() }

func (u *UUID) Attr(name string) (starlark.Value, error) {
	switch name {
	case "hex":
		// Remove hyphens from the UUID string
		var hex strings.Builder
		for _, c := range string(u.StringUUID) {
			if c != '-' {
				hex.WriteString(string(c))
			}
		}
		return starlark.String(hex.String()), nil
	case "urn":
		return starlark.String("urn:uuid:" + string(u.StringUUID)), nil
	default:
		return nil, nil
	}
}

func (u *UUID) AttrNames() []string {
	return []string{"hex", "urn"}
}
