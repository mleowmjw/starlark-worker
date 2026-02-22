package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
	_ "modernc.org/sqlite"
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

	db, err := openDB(dbPath.GoString())
	if err != nil {
		logger.Error("sqlite.exec: open db failed", "error", err)
		return nil, err
	}
	defer db.Close()

	ctx := safeclaw.GetContext(t)
	res, err := db.ExecContext(ctx, sqlStmt.GoString())
	if err != nil {
		logger.Error("sqlite.exec: exec failed", "error", err)
		return nil, err
	}

	rows, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()

	out := starlark.NewDict(2)
	_ = out.SetKey(starlark.String("rows_affected"), starlark.MakeInt64(rows))
	_ = out.SetKey(starlark.String("last_insert_id"), starlark.MakeInt64(lastID))
	return out, nil
}

func querySQL(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var dbPath, sqlStmt starlark.String
	if err := starlark.UnpackArgs("query", args, kwargs, "db_path", &dbPath, "sql", &sqlStmt); err != nil {
		logger.Error("sqlite.query: unpack args failed", "error", err)
		return nil, err
	}

	db, err := openDB(dbPath.GoString())
	if err != nil {
		logger.Error("sqlite.query: open db failed", "error", err)
		return nil, err
	}
	defer db.Close()

	ctx := safeclaw.GetContext(t)
	rows, err := db.QueryContext(ctx, sqlStmt.GoString())
	if err != nil {
		logger.Error("sqlite.query: query failed", "error", err)
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		logger.Error("sqlite.query: columns failed", "error", err)
		return nil, err
	}

	result := starlark.NewList(nil)
	for rows.Next() {
		values := make([]any, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range values {
			scanArgs[i] = &values[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			logger.Error("sqlite.query: scan failed", "error", err)
			return nil, err
		}

		rowDict := starlark.NewDict(len(cols))
		for i, name := range cols {
			_ = rowDict.SetKey(starlark.String(name), toStarlarkValue(values[i]))
		}
		result.Append(rowDict)
	}
	if err := rows.Err(); err != nil {
		logger.Error("sqlite.query: rows error", "error", err)
		return nil, err
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
