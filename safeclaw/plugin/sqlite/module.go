package sqlite

import (
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/nondet"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/temporalops"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type Module struct{}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "sqlite" }
func (f *Module) Type() string                          { return "sqlite" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: sqlite") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"exec":  starlark.NewBuiltin("exec", execSQL),
	"query": starlark.NewBuiltin("query", querySQL),
}

var properties = map[string]star.PropertyFactory{}

func execSQL(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var dbPath, sqlStmt starlark.String
	if err := starlark.UnpackArgs("exec", args, kwargs, "db_path", &dbPath, "sql", &sqlStmt); err != nil {
		logger.Error("sqlite.exec: unpack args failed", "error", err)
		return nil, err
	}

	ctx := safeclaw.GetContext(t)
	res, err := nondet.SQLiteExec(ctx, safeclaw.GetRuntime(t), temporalops.SQLiteExecInput{
		DBPath: dbPath.GoString(),
		SQL:    sqlStmt.GoString(),
	})
	if err != nil {
		logger.Error("sqlite.exec: exec failed", "error", err)
		return nil, err
	}

	out := starlark.NewDict(2)
	_ = out.SetKey(starlark.String("rows_affected"), starlark.MakeInt64(res.RowsAffected))
	_ = out.SetKey(starlark.String("last_insert_id"), starlark.MakeInt64(res.LastInsertID))
	return out, nil
}

func querySQL(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var dbPath, sqlStmt starlark.String
	if err := starlark.UnpackArgs("query", args, kwargs, "db_path", &dbPath, "sql", &sqlStmt); err != nil {
		logger.Error("sqlite.query: unpack args failed", "error", err)
		return nil, err
	}

	ctx := safeclaw.GetContext(t)
	out, err := nondet.SQLiteQuery(ctx, safeclaw.GetRuntime(t), temporalops.SQLiteQueryInput{
		DBPath: dbPath.GoString(),
		SQL:    sqlStmt.GoString(),
	})
	if err != nil {
		logger.Error("sqlite.query: query failed", "error", err)
		return nil, err
	}

	result := starlark.NewList(nil)
	for _, row := range out.Rows {
		rowDict := starlark.NewDict(len(row))
		for name, value := range row {
			_ = rowDict.SetKey(starlark.String(name), toStarlarkValue(value))
		}
		result.Append(rowDict)
	}

	return result, nil
}

func toStarlarkValue(v interface{}) starlark.Value {
	switch t := v.(type) {
	case nil:
		return starlark.None
	case []byte:
		return starlark.String(string(t))
	case string:
		return starlark.String(t)
	case int64:
		return starlark.MakeInt64(t)
	case float64:
		return starlark.Float(t)
	case bool:
		return starlark.Bool(t)
	default:
		return starlark.String(fmt.Sprint(t))
	}
}
