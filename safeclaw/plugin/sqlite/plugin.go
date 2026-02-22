package sqlite

import (
	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "sqlite"
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	backend := workflow.GetBackend(ctx)
	return &Module{
		backend: backend,
	}
}
