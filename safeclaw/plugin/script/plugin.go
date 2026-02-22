package script

import (
	"context"
	"fmt"
	"strings"

	"github.com/bitfield/script"
	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/mode"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/nondet"
	"github.com/cadence-workflow/starlark-worker/safeclaw/runtime/temporalops"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "script"
}

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	return &Module{
		ctx:     ctx,
		mode:    info.Mode,
		runtime: info.Runtime,
	}
}

type Module struct {
	ctx     context.Context
	mode    mode.Value
	runtime nondet.Runtime
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "script" }
func (f *Module) Type() string                          { return "script" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: script") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"exec":  starlark.NewBuiltin("exec", _exec),
	"file":  starlark.NewBuiltin("file", _file),
	"echo":  starlark.NewBuiltin("echo", _echo),
	"stdin": starlark.NewBuiltin("stdin", _stdin),
}

var properties = map[string]star.PropertyFactory{}

func _exec(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var command starlark.String
	if err := starlark.UnpackArgs("exec", args, kwargs, "command", &command); err != nil {
		logger.Error("script.exec: unpack args failed", "error", err)
		return nil, err
	}

	module := getScriptModule(t)
	if module.mode == mode.Dev {
		return &Pipe{pipe: script.Exec(command.GoString()), mode: module.mode, runtime: module.runtime, ctx: module.ctx}, nil
	}
	return &Pipe{
		mode:    module.mode,
		runtime: module.runtime,
		ctx:     module.ctx,
		source:  pipeSource{kind: sourceExec, value: command.GoString()},
	}, nil
}

func _file(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var path starlark.String
	if err := starlark.UnpackArgs("file", args, kwargs, "path", &path); err != nil {
		logger.Error("script.file: unpack args failed", "error", err)
		return nil, err
	}

	module := getScriptModule(t)
	if module.mode == mode.Dev {
		return &Pipe{pipe: script.File(path.GoString()), mode: module.mode, runtime: module.runtime, ctx: module.ctx}, nil
	}
	return &Pipe{
		mode:    module.mode,
		runtime: module.runtime,
		ctx:     module.ctx,
		source:  pipeSource{kind: sourceFile, value: path.GoString()},
	}, nil
}

func _echo(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var text starlark.String
	if err := starlark.UnpackArgs("echo", args, kwargs, "text", &text); err != nil {
		logger.Error("script.echo: unpack args failed", "error", err)
		return nil, err
	}

	module := getScriptModule(t)
	if module.mode == mode.Dev {
		return &Pipe{pipe: script.Echo(text.GoString()), mode: module.mode, runtime: module.runtime, ctx: module.ctx}, nil
	}
	return &Pipe{
		mode:    module.mode,
		runtime: module.runtime,
		ctx:     module.ctx,
		source:  pipeSource{kind: sourceEcho, value: text.GoString()},
	}, nil
}

func _stdin(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	module := getScriptModule(t)
	if module.mode == mode.Dev {
		return &Pipe{pipe: script.Stdin(), mode: module.mode, runtime: module.runtime, ctx: module.ctx}, nil
	}
	return nil, fmt.Errorf("script.stdin is not available in non-dev modes")
}

// Pipe wraps script.Pipe for Starlark
type Pipe struct {
	pipe    *script.Pipe
	mode    mode.Value
	runtime nondet.Runtime
	ctx     context.Context
	source  pipeSource
	ops     []pipeOp
}

type sourceKind string

const (
	sourceExec sourceKind = "exec"
	sourceFile sourceKind = "file"
	sourceEcho sourceKind = "echo"
)

type pipeSource struct {
	kind  sourceKind
	value string
}

type opKind string

const (
	opExec    opKind = "exec"
	opMatch   opKind = "match"
	opReplace opKind = "replace"
)

type pipeOp struct {
	kind opKind
	a    string
	b    string
}

var _ starlark.Value = &Pipe{}
var _ starlark.HasAttrs = &Pipe{}

func (p *Pipe) String() string        { return "<Pipe>" }
func (p *Pipe) Type() string          { return "Pipe" }
func (p *Pipe) Freeze()               {}
func (p *Pipe) Truth() starlark.Bool  { return true }
func (p *Pipe) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Pipe") }

func (p *Pipe) Attr(name string) (starlark.Value, error) {
	switch name {
	case "string":
		return starlark.NewBuiltin("string", p.stringMethod), nil
	case "bytes":
		return starlark.NewBuiltin("bytes", p.bytesMethod), nil
	case "lines":
		return starlark.NewBuiltin("lines", p.linesMethod), nil
	case "exec":
		return starlark.NewBuiltin("exec", p.execMethod), nil
	case "match_regexp":
		return starlark.NewBuiltin("match_regexp", p.matchRegexpMethod), nil
	case "replace":
		return starlark.NewBuiltin("replace", p.replaceMethod), nil
	default:
		return nil, nil
	}
}

func (p *Pipe) AttrNames() []string {
	return []string{"string", "bytes", "lines", "exec", "match_regexp", "replace"}
}

func (p *Pipe) stringMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	resultBytes, err := p.evalBytes()
	if err != nil {
		return nil, err
	}
	return starlark.String(string(resultBytes)), nil
}

func (p *Pipe) bytesMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	result, err := p.evalBytes()
	if err != nil {
		return nil, err
	}
	return starlark.Bytes(result), nil
}

func (p *Pipe) linesMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	resultBytes, err := p.evalBytes()
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSuffix(string(resultBytes), "\n")
	if trimmed == "" {
		return starlark.NewList(nil), nil
	}
	result := strings.Split(trimmed, "\n")
	lines := make([]starlark.Value, len(result))
	for i, line := range result {
		lines[i] = starlark.String(line)
	}
	return starlark.NewList(lines), nil
}

func (p *Pipe) execMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var command starlark.String
	if err := starlark.UnpackArgs("exec", args, kwargs, "command", &command); err != nil {
		logger.Error("pipe.exec: unpack args failed", "error", err)
		return nil, err
	}

	if p.mode == mode.Dev {
		newPipe := p.pipe.Exec(command.GoString())
		return &Pipe{pipe: newPipe, mode: p.mode, runtime: p.runtime, ctx: p.ctx}, nil
	}
	cp := p.clone()
	cp.ops = append(cp.ops, pipeOp{kind: opExec, a: command.GoString()})
	return cp, nil
}

func (p *Pipe) matchRegexpMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var pattern starlark.String
	if err := starlark.UnpackArgs("match_regexp", args, kwargs, "pattern", &pattern); err != nil {
		logger.Error("pipe.match_regexp: unpack args failed", "error", err)
		return nil, err
	}

	if p.mode == mode.Dev {
		newPipe := p.pipe.Match(pattern.GoString())
		return &Pipe{pipe: newPipe, mode: p.mode, runtime: p.runtime, ctx: p.ctx}, nil
	}
	cp := p.clone()
	cp.ops = append(cp.ops, pipeOp{kind: opMatch, a: pattern.GoString()})
	return cp, nil
}

func (p *Pipe) replaceMethod(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var old, new starlark.String
	if err := starlark.UnpackArgs("replace", args, kwargs, "old", &old, "new", &new); err != nil {
		logger.Error("pipe.replace: unpack args failed", "error", err)
		return nil, err
	}

	if p.mode == mode.Dev {
		newPipe := p.pipe.Replace(old.GoString(), new.GoString())
		return &Pipe{pipe: newPipe, mode: p.mode, runtime: p.runtime, ctx: p.ctx}, nil
	}
	cp := p.clone()
	cp.ops = append(cp.ops, pipeOp{kind: opReplace, a: old.GoString(), b: new.GoString()})
	return cp, nil
}

func (p *Pipe) clone() *Pipe {
	cp := &Pipe{
		mode:    p.mode,
		runtime: p.runtime,
		ctx:     p.ctx,
		source:  p.source,
	}
	if len(p.ops) > 0 {
		cp.ops = append([]pipeOp(nil), p.ops...)
	}
	return cp
}

func (p *Pipe) evalBytes() ([]byte, error) {
	if p.mode == mode.Dev {
		return p.pipe.Bytes()
	}

	var b []byte
	var source temporalops.ScriptSource
	switch p.source.kind {
	case sourceExec:
		source = temporalops.ScriptSource{Kind: "exec", Data: p.source.value}
	case sourceFile:
		source = temporalops.ScriptSource{Kind: "file", Data: p.source.value}
	case sourceEcho:
		source = temporalops.ScriptSource{Kind: "echo", Data: p.source.value}
	default:
		return nil, fmt.Errorf("unknown pipe source: %s", p.source.kind)
	}
	ops := make([]temporalops.ScriptOp, 0, len(p.ops))
	for _, op := range p.ops {
		switch op.kind {
		case opExec:
			ops = append(ops, temporalops.ScriptOp{Kind: "exec", A: op.a})
		case opMatch:
			ops = append(ops, temporalops.ScriptOp{Kind: "match", A: op.a})
		case opReplace:
			ops = append(ops, temporalops.ScriptOp{Kind: "replace", A: op.a, B: op.b})
		default:
			return nil, fmt.Errorf("unsupported pipe operation: %s", op.kind)
		}
	}
	var err error
	b, err = nondet.ScriptPipeline(p.ctx, p.runtime, temporalops.ScriptPipelineInput{
		Source: source,
		Ops:    ops,
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

func getScriptModule(t *starlark.Thread) *Module {
	if m, ok := t.Local("script_module").(*Module); ok && m != nil {
		return m
	}
	return &Module{
		ctx:     safeclaw.GetContext(t),
		mode:    safeclaw.GetRuntime(t).Mode(),
		runtime: safeclaw.GetRuntime(t),
	}
}
