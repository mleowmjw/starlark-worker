package sqlite

import (
	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

var _ safeclaw.Registrar = (*plugin)(nil)

func (p *plugin) ID() string {
	return "sqlite"
}

// RegisterActivities registers the SQLite activities with the Temporal worker.
func (p *plugin) RegisterActivities(registerFn func(activity any)) {
	registerFn(SQLExecActivity)
	registerFn(SQLQueryActivity)
}

func (p *plugin) Module(ctx any, info safeclaw.RunInfo) starlark.Value {
	backend := workflow.GetBackend(ctx)
	return &Module{
		backend: backend,
	}
}
