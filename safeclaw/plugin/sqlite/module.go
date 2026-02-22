package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
	"go.starlark.net/starlark"
	_ "modernc.org/sqlite"
)

type Module struct {
	backend workflow.Backend
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string        { return "sqlite" }
func (f *Module) Type() string          { return "sqlite" }
func (f *Module) Freeze()               {}
func (f *Module) Truth() starlark.Bool  { return true }
func (f *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: sqlite") }
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
		"exec":  starlark.NewBuiltin("exec", execSQL).BindReceiver(m),
		"query": starlark.NewBuiltin("query", querySQL).BindReceiver(m),
	}
}

var properties = map[string]star.PropertyFactory{}

func execSQL(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)
	receiver := fn.Receiver().(*Module)

	var dbPath, sqlStmt starlark.String
	if err := starlark.UnpackArgs("exec", args, kwargs, "db_path", &dbPath, "sql", &sqlStmt); err != nil {
		logger.Error("sqlite.exec: unpack args failed", "error", err)
		return nil, err
	}

	// Execute via activity (works for both local and workflow backends)
	var output SQLExecOutput
	err := receiver.backend.ExecuteActivity(SQLExecActivity, SQLExecInput{
		DBPath: dbPath.GoString(),
		SQL:    sqlStmt.GoString(),
	}).Get(&output)
	if err != nil {
		logger.Error("sqlite.exec: activity failed", "error", err)
		return nil, err
	}

	out := starlark.NewDict(2)
	_ = out.SetKey(starlark.String("rows_affected"), starlark.MakeInt64(output.RowsAffected))
	_ = out.SetKey(starlark.String("last_insert_id"), starlark.MakeInt64(output.LastInsertID))
	return out, nil
}

func querySQL(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)
	receiver := fn.Receiver().(*Module)

	var dbPath, sqlStmt starlark.String
	if err := starlark.UnpackArgs("query", args, kwargs, "db_path", &dbPath, "sql", &sqlStmt); err != nil {
		logger.Error("sqlite.query: unpack args failed", "error", err)
		return nil, err
	}

	// Execute via activity (works for both local and workflow backends)
	var output SQLQueryOutput
	err := receiver.backend.ExecuteActivity(SQLQueryActivity, SQLQueryInput{
		DBPath: dbPath.GoString(),
		SQL:    sqlStmt.GoString(),
	}).Get(&output)
	if err != nil {
		logger.Error("sqlite.query: activity failed", "error", err)
		return nil, err
	}

	result := starlark.NewList(nil)
	for _, row := range output.Rows {
		rowDict := starlark.NewDict(len(output.Columns))
		for i, name := range output.Columns {
			_ = rowDict.SetKey(starlark.String(name), toStarlarkValue(row[i]))
		}
		result.Append(rowDict)
	}
	return result, nil
}

func openDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("db_path is required")
	}
	if err := ensureDir(dbPath); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", dbPath)
}

func ensureDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func toStarlarkValue(v any) starlark.Value {
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
