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
	"regexp"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	OpNow            = "now"
	OpSleep          = "sleep"
	OpRandInt        = "rand_int"
	OpRandFloat      = "rand_float"
	OpUUID4          = "uuid4"
	OpHTTPRequestDo  = "http_do"
	OpSQLiteExec     = "sqlite_exec"
	OpSQLiteQuery    = "sqlite_query"
	OpScriptExec     = "script_exec"
	OpScriptFile     = "script_file"
	OpScriptPipeline = "script_pipeline"
)

type OperationRequest struct {
	Op           string       `json:"op"`
	Payload      []byte       `json:"payload,omitempty"`
	ScriptPolicy ScriptPolicy `json:"script_policy,omitempty"`
}

type NowOutput struct {
	UnixNano int64 `json:"unix_nano"`
}

type SleepInput struct {
	DurationNanos int64 `json:"duration_nanos"`
}

type RandIntInput struct {
	Min     int    `json:"min"`
	Max     int    `json:"max"`
	Seed    *int64 `json:"seed,omitempty"`
	Counter uint64 `json:"counter,omitempty"`
}

type RandIntOutput struct {
	Value int `json:"value"`
}

type RandFloatOutput struct {
	Value float64 `json:"value"`
}

type RandFloatInput struct {
	Seed    *int64 `json:"seed,omitempty"`
	Counter uint64 `json:"counter,omitempty"`
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

type ScriptExecInput struct {
	Command string `json:"command"`
	Stdin   []byte `json:"stdin,omitempty"`
}

type ScriptFileInput struct {
	Path string `json:"path"`
}

type ScriptOutput struct {
	Bytes []byte `json:"bytes"`
}

type ScriptPipelineInput struct {
	Source ScriptSource `json:"source"`
	Ops    []ScriptOp   `json:"ops,omitempty"`
}

type ScriptSource struct {
	Kind string `json:"kind"`
	Data string `json:"data,omitempty"`
}

type ScriptOp struct {
	Kind string `json:"kind"`
	A    string `json:"a,omitempty"`
	B    string `json:"b,omitempty"`
}

func ExecuteLocal(ctx context.Context, op string, payload []byte) ([]byte, error) {
	return ExecuteLocalWithPolicy(ctx, op, payload, ScriptPolicy{})
}

func ExecuteLocalWithPolicy(ctx context.Context, op string, payload []byte, policy ScriptPolicy) ([]byte, error) {
	allow := normalizeAllowlist(policy.Allowlist)
	if len(allow) == 0 {
		allow = normalizeAllowlist(DefaultScriptAllowlist())
	}

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
		var v int
		if in.Seed != nil {
			r := rand.New(rand.NewPCG(uint64(*in.Seed)+in.Counter, uint64(*in.Seed)^in.Counter))
			v = r.IntN(in.Max-in.Min+1) + in.Min
		} else {
			v = rand.IntN(in.Max-in.Min+1) + in.Min
		}
		out := RandIntOutput{Value: v}
		return json.Marshal(out)
	case OpRandFloat:
		var in RandFloatInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, err
			}
		}
		var v float64
		if in.Seed != nil {
			r := rand.New(rand.NewPCG(uint64(*in.Seed)+in.Counter, uint64(*in.Seed)^in.Counter))
			v = r.Float64()
		} else {
			v = rand.Float64()
		}
		out := RandFloatOutput{Value: v}
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
	case OpScriptExec:
		var in ScriptExecInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		commandName := firstCommandName(in.Command)
		if commandName == "" {
			return nil, fmt.Errorf("script command is empty")
		}
		if _, ok := allow[strings.ToLower(commandName)]; !ok {
			return nil, fmt.Errorf("script command not allowed in non-dev mode: %s", commandName)
		}
		var p *script.Pipe
		if len(in.Stdin) > 0 {
			p = script.Echo(string(in.Stdin)).Exec(in.Command)
		} else {
			p = script.Exec(in.Command)
		}
		out, err := p.Bytes()
		if err != nil {
			return nil, err
		}
		return json.Marshal(ScriptOutput{Bytes: out})
	case OpScriptFile:
		var in ScriptFileInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		out, err := script.File(in.Path).Bytes()
		if err != nil {
			return nil, err
		}
		return json.Marshal(ScriptOutput{Bytes: out})
	case OpScriptPipeline:
		var in ScriptPipelineInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, err
		}
		out, err := executeScriptPipeline(in, allow)
		if err != nil {
			return nil, err
		}
		return json.Marshal(ScriptOutput{Bytes: out})
	default:
		return nil, fmt.Errorf("unsupported nondet operation: %s", op)
	}
}

var commandNamePattern = regexp.MustCompile(`^\s*([A-Za-z0-9._/-]+)`)

func firstCommandName(command string) string {
	m := commandNamePattern.FindStringSubmatch(command)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func executeScriptPipeline(in ScriptPipelineInput, allow map[string]struct{}) ([]byte, error) {
	var p *script.Pipe
	switch in.Source.Kind {
	case "exec":
		cmd := firstCommandName(in.Source.Data)
		if cmd == "" {
			return nil, fmt.Errorf("script command is empty")
		}
		if _, ok := allow[strings.ToLower(cmd)]; !ok {
			return nil, fmt.Errorf("script command not allowed in non-dev mode: %s", cmd)
		}
		p = script.Exec(in.Source.Data)
	case "file":
		p = script.File(in.Source.Data)
	case "echo":
		p = script.Echo(in.Source.Data)
	default:
		return nil, fmt.Errorf("unsupported script source kind: %s", in.Source.Kind)
	}

	for _, op := range in.Ops {
		switch op.Kind {
		case "exec":
			cmd := firstCommandName(op.A)
			if cmd == "" {
				return nil, fmt.Errorf("script command is empty")
			}
			if _, ok := allow[strings.ToLower(cmd)]; !ok {
				return nil, fmt.Errorf("script command not allowed in non-dev mode: %s", cmd)
			}
			p = p.Exec(op.A)
		case "match":
			p = p.Match(op.A)
		case "replace":
			p = p.Replace(op.A, op.B)
		default:
			return nil, fmt.Errorf("unsupported script op kind: %s", op.Kind)
		}
	}

	return p.Bytes()
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
