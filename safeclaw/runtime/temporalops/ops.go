package temporalops

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	OpNow           = "now"
	OpSleep         = "sleep"
	OpRandInt       = "rand_int"
	OpRandFloat     = "rand_float"
	OpUUID4         = "uuid4"
	OpHTTPRequestDo = "http_do"
	OpSQLiteExec    = "sqlite_exec"
	OpSQLiteQuery   = "sqlite_query"
)

type OperationRequest struct {
	Op      string `json:"op"`
	Payload []byte `json:"payload,omitempty"`
}

type NowOutput struct {
	UnixNano int64 `json:"unix_nano"`
}

type SleepInput struct {
	DurationNanos int64 `json:"duration_nanos"`
}

type RandIntInput struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type RandIntOutput struct {
	Value int `json:"value"`
}

type RandFloatOutput struct {
	Value float64 `json:"value"`
}

type UUIDOutput struct {
	Value string `json:"value"`
}

type HTTPRequestInput struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Body    []byte              `json:"body,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type HTTPResponseOutput struct {
	ResponseBytes []byte `json:"response_bytes"`
}

type SQLiteExecInput struct {
	DBPath string `json:"db_path"`
	SQL    string `json:"sql"`
}

type SQLiteExecOutput struct {
	RowsAffected int64 `json:"rows_affected"`
	LastInsertID int64 `json:"last_insert_id"`
}

type SQLiteQueryInput struct {
	DBPath string `json:"db_path"`
	SQL    string `json:"sql"`
}

type SQLiteQueryOutput struct {
	Rows []map[string]any `json:"rows"`
}

func ExecuteLocal(ctx context.Context, op string, payload []byte) ([]byte, error) {
	switch op {
	case OpNow:
		out := NowOutput{UnixNano: time.Now().UnixNano()}
		return json.Marshal(out)
	case OpSleep:
		var in SleepInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		d := time.Duration(in.DurationNanos)
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			return []byte("{}"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case OpRandInt:
		var in RandIntInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		out := RandIntOutput{Value: rand.IntN(in.Max-in.Min+1) + in.Min}
		return json.Marshal(out)
	case OpRandFloat:
		out := RandFloatOutput{Value: rand.Float64()}
		return json.Marshal(out)
	case OpUUID4:
		out := UUIDOutput{Value: uuid.New().String()}
		return json.Marshal(out)
	case OpHTTPRequestDo:
		var in HTTPRequestInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		var br io.Reader
		if len(in.Body) > 0 {
			br = bytes.NewReader(in.Body)
		}
		req, err := http.NewRequestWithContext(ctx, in.Method, in.URL, br)
		if err != nil {
			return nil, err
		}
		req.Header = http.Header(in.Headers)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		var bb bytes.Buffer
		if err := res.Write(&bb); err != nil {
			return nil, err
		}
		out := HTTPResponseOutput{ResponseBytes: bb.Bytes()}
		return json.Marshal(out)
	case OpSQLiteExec:
		var in SQLiteExecInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		db, err := openDB(in.DBPath)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		res, err := db.ExecContext(ctx, in.SQL)
		if err != nil {
			return nil, err
		}
		rows, _ := res.RowsAffected()
		lastID, _ := res.LastInsertId()
		return json.Marshal(SQLiteExecOutput{RowsAffected: rows, LastInsertID: lastID})
	case OpSQLiteQuery:
		var in SQLiteQueryInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		db, err := openDB(in.DBPath)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		rows, err := db.QueryContext(ctx, in.SQL)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		outRows := make([]map[string]any, 0)
		for rows.Next() {
			vals := make([]any, len(cols))
			args := make([]any, len(cols))
			for i := range vals {
				args[i] = &vals[i]
			}
			if err := rows.Scan(args...); err != nil {
				return nil, err
			}
			row := make(map[string]any, len(cols))
			for i, c := range cols {
				row[c] = normalizeSQLValue(vals[i])
			}
			outRows = append(outRows, row)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return json.Marshal(SQLiteQueryOutput{Rows: outRows})
	default:
		return nil, fmt.Errorf("unsupported nondet operation: %s", op)
	}
}

func DecodeHTTPResponse(raw []byte) (*http.Response, error) {
	var out HTTPResponseOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewReader(out.ResponseBytes)), nil)
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

func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case string, int64, float64, bool:
		return t
	default:
		return fmt.Sprint(t)
	}
}
