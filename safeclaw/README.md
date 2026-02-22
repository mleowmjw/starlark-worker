# Safeclaw: Hermetic Starlark Execution Core

Safeclaw is a standalone Go module that provides safe, hermetic execution of Starlark scripts. It extracts the core Starlark execution engine from `starlark-worker`, with mode-aware runtime routing for nondeterministic operations.

## Features

- **Go 1.26**: Uses modern stdlib features like `log/slog`, `context.Context`, `errors.Join`, `encoding/json`
- **12 Built-in Plugins**: json, time, random, uuid, os, hashlib, test, atexit, progress, concurrent, request, script
- **Hermetic Execution**: Sandboxed Starlark environment with controlled capabilities
- **Extensible**: Easy to create custom plugins
- **Safe for Agents**: Ideal for AI agent tool execution with structured I/O

## Installation

### Prerequisites

- Go 1.26 or later
- Git

### Install SafeClaw

```bash
# Clone the repository
git clone https://github.com/cadence-workflow/starlark-worker.git
cd starlark-worker/safeclaw

# Download dependencies
go mod tidy
```

### Verify Installation

```bash
# Run tests
go test ./...

# Run an example
go run ./examples/01_hello/main.go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cadence-workflow/starlark-worker/safeclaw"
    "github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

func main() {
    // Create a runner with default plugins
    runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

    // Execute Starlark code
    source := []byte(`
def greet(name):
    return "Hello, " + name + "!"
`)

    result, err := runner.RunSource(context.Background(), source, "greet", "World")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // Output: "Hello, World!"
}
```

### Three Ways to Run Scripts

#### 1. Inline Source (RunSource)

```go
source := []byte(`def add(a, b): return a + b`)
result, err := runner.RunSource(ctx, source, "add", 5, 3)
```

#### 2. From Filesystem (RunScript)

```go
fs := star.NewLocalFS("/path/to/scripts")
result, err := runner.RunScript(ctx, fs, "script.star", "function", args...)
```

#### 3. From Tar Archive (RunTar)

```go
tarData := []byte{...} // gzipped tar archive
result, err := runner.RunTar(ctx, tarData, "/main.star", "function", args...)
```

### Runtime Modes and RunOptions

Safeclaw resolves runtime mode from `CHAMELEON_MODE` or `SAFECLAW_ENV`:

- `dev`: local stdlib execution
- `staging|prod`: Temporal SDK-backed execution
- `test`: Temporal testsuite-backed deterministic execution

Use `RunSourceWithOptions`, `RunScriptWithOptions`, or `RunTarWithOptions` to pass per-run settings:

```go
fixed := int64(1700000000000000000)
result, err := runner.RunSourceWithOptions(ctx, source, "run", safeclaw.RunOptions{
    Environ: map[string]string{"SAFECLAW_ENV": "test"},
    Test: safeclaw.TestRuntimeOptions{
        FixedUnixNano: &fixed,
    },
    Script: safeclaw.ScriptRuntimeOptions{
        Allowlist: []string{"tr", "sed"},
    },
})
```

`Script.Allowlist` is enforced in non-dev modes by the runtime execution layer, not by plugin-local checks.

## Architecture

```
safeclaw/
├── safeclaw.go          # Core Runner and Plugin interface
├── star/                # Starlark utilities (codec, fs, dataclass)
├── ext/                 # Helper utilities (tar, json_path, etc.)
├── plugin/              # Plugin implementations
│   ├── registry.go      # Default plugin set
│   ├── json/            # JSON encoding/decoding
│   ├── time/            # Time operations
│   ├── random/          # Random number generation
│   ├── uuid/            # UUID generation
│   ├── os/              # Environment variables
│   ├── hashlib/         # Cryptographic hashing
│   ├── test/            # Assertions
│   ├── atexit/          # Exit hooks
│   ├── progress/        # Progress reporting
│   ├── concurrent/      # Goroutine-backed concurrency
│   ├── request/         # HTTP client
│   └── script/          # Shell-like operations (bitfield/script)
└── examples/            # 10 diverse examples
```

## Plugins

### json
```python
load("@plugin", "json")
obj = {"name": "test", "value": 42}
encoded = json.dumps(obj)
decoded = json.loads(encoded)
```

### time
```python
load("@plugin", "time")
current = time.time()
time.sleep(seconds=1.5)
formatted = time.utc_format_seconds(format="%Y-%m-%d", seconds=current)
```

### random
```python
load("@plugin", "random")
random.seed(12345)
val = random.randint(min=1, max=100)
flt = random.random()
```

### uuid
```python
load("@plugin", "uuid")
u = uuid.uuid4()
print(u.hex)  # Without hyphens
print(u.urn)  # URN format
```

### os
```python
load("@plugin", "os")
app_name = os.environ.get("APP_NAME", "default")
```

### hashlib
```python
load("@plugin", "hashlib")
hash_val = hashlib.blake2b_hex(data="test", digest_size=16)
```

### test
```python
load("@plugin", t = "test")
t.true(value > 0)
t.equal(expected=5, actual=result)
```

### concurrent
```python
load("@plugin", "concurrent")

def worker(n):
    return n * 2

futures = [concurrent.run(worker, i) for i in range(5)]
results = [f.result() for f in futures]
```

### request
```python
load("@plugin", "request")
response = request.do(method="GET", url="https://api.example.com/data")
data = response.json
```

### script
```python
load("@plugin", "script")
result = script.exec("echo hello | tr a-z A-Z").string()
lines = script.file("/etc/hosts").lines()
```

## Custom Plugins

```go
type MyPlugin struct{}

func (p *MyPlugin) ID() string {
    return "myplugin"
}

func (p *MyPlugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
    return &MyModule{}
}

// Implement starlark.HasAttrs for MyModule...

// Register with runner
plugins := append(plugin.DefaultPlugins(), &MyPlugin{})
runner := safeclaw.NewRunner(plugins, nil)
```

## Running Examples

The `examples/` directory contains 10 complete, runnable examples demonstrating SafeClaw's capabilities.

### Quick Start - Run Any Example

```bash
# From the safeclaw directory
cd examples/01_hello
go run main.go
```

Or run directly from safeclaw root:
```bash
go run ./examples/01_hello/main.go
```

### All Examples

#### 1. Basic Execution (`01_hello`)
**What it demonstrates**: Simple function calls with arguments and return values

```bash
cd examples/01_hello && go run main.go
```

**Expected output**:
```
Result: "Hello, World!"
```

**Key concepts**: Runner creation, inline scripts, function execution

---

#### 2. JSON Processing (`02_json`)
**What it demonstrates**: Parsing JSON, data transformation, list comprehensions

```bash
cd examples/02_json && go run main.go
```

**Expected output**:
```
Result: "{\"count\": 3, \"names\": [\"apple\", \"banana\", \"cherry\"], \"total_value\": 60}"
```

**Key concepts**: Plugin loading, JSON manipulation, data aggregation

---

#### 3. HTTP Requests (`03_http`)
**What it demonstrates**: Making HTTP requests and processing responses

```bash
cd examples/03_http && go run main.go
```

**Expected output**:
```
Result: {"status": 200, "message": "Hello from server", "method": "GET"}
```

**Key concepts**: HTTP client, test server, combining plugins (request + json)

---

#### 4. Concurrent Execution (`04_concurrent`)
**What it demonstrates**: Running tasks in parallel with futures

```bash
cd examples/04_concurrent && go run main.go
```

**Expected output**:
```
Result: ["Task 0 completed", "Task 1 completed", "Task 2 completed", "Task 3 completed", "Task 4 completed"]
```

**Key concepts**: Goroutine-backed concurrency, future pattern, result collection

---

#### 5. ETL Pipeline (`05_pipeline`)
**What it demonstrates**: Multi-stage data processing (Extract-Transform-Load)

```bash
cd examples/05_pipeline && go run main.go
```

**Expected output**:
```
Result: "{\"items\": [{\"hash\": \"...\", \"id\": 1, \"name\": \"ALICE\"}, ...], \"total\": 3}"
```

**Key concepts**: Function composition, data transformation, hashing

---

#### 6. Environment Variables (`06_environ`)
**What it demonstrates**: Accessing environment configuration

```bash
cd examples/06_environ && go run main.go
```

**Expected output**:
```
Result: {"app_name": "", "current_time": 1707264000000000000, "debug": ""}

Note: This example demonstrates environment variable access.
The STARLARK_TIME env var allows backfilling scripts to a specific time.
```

**Key concepts**: RunInfo context, environment variables, time backfilling

---

#### 7. Tar Archives (`07_files`)
**What it demonstrates**: Loading scripts from tar archives with dependencies

```bash
cd examples/07_files && go run main.go
```

**Expected output**:
```
Result: {"config": {"enabled": True, "version": "1.0"}, "data": "Hello from a text file!", "message": "Processed 23 bytes of data"}
```

**Key concepts**: TarFS, multi-file scripts, loading JSON/text data files

---

#### 8. Shell Operations (`08_script`)
**What it demonstrates**: Shell-like text processing with pipelines

```bash
cd examples/08_script && go run main.go
```

**Expected output**:
```
Result: {"line_count": 3, "replaced": "foo BAR baz\n", "uppercase": "HELLO WORLD\n"}
```

**Key concepts**: Script plugin, pipeline pattern, text transformation

---

#### 9. Custom Plugin (`09_custom_plugin`)
**What it demonstrates**: Creating and using custom plugins

```bash
cd examples/09_custom_plugin && go run main.go
```

**Expected output**:
```
Result: {"greeting": "Hello, World, from custom plugin!", "product": 42, "version": "1.0.0"}
```

**Key concepts**: Plugin interface, custom builtins, property accessors

---

#### 10. Agent Tool Executor (`10_agent_tool`) ⭐
**What it demonstrates**: SafeClaw as a safe runtime for AI agent tools

```bash
cd examples/10_agent_tool && go run main.go
```

**Expected output**:
```
Tool Execution Result:
{
  "success": true,
  "result": "{\"data\": {...}, \"status\": \"success\"}"
}

=== Agent Tool Executor Demo ===
This example demonstrates using Starlark as a safe, hermetic
execution environment for agent tools. Key features:
- Sandboxed execution (no direct filesystem/network access)
- Controlled capabilities via plugins
- Structured input/output
- Safe for untrusted code execution
```

**Key concepts**: Tool specification, structured I/O, safe execution, **primary use case for AI agents**

---

### Running All Examples

Run all examples sequentially:

```bash
# From safeclaw directory
for dir in examples/*/; do
    echo "=== Running $(basename $dir) ==="
    (cd "$dir" && go run main.go)
    echo ""
done
```

Or create a test script:

```bash
#!/bin/bash
# run_all_examples.sh

cd "$(dirname "$0")"

for i in {01..10}; do
    example_dir="examples/${i}_"*
    if [ -d "$example_dir" ]; then
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "Running: $(basename "$example_dir")"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        (cd "$example_dir" && go run main.go) || echo "❌ Failed"
        echo ""
    fi
done
```

### Troubleshooting Examples

**Issue**: `package github.com/cadence-workflow/starlark-worker/safeclaw is not in GOROOT`

**Solution**: Make sure you're in the safeclaw directory and dependencies are downloaded:
```bash
cd /path/to/starlark-worker/safeclaw
go mod tidy
go mod download
```

**Issue**: Network errors in example 03 or 10

**Solution**: These examples make real HTTP requests. Ensure you have network connectivity.

**Issue**: Permission errors in example 08 (script)

**Solution**: The script plugin executes shell commands. Ensure the commands exist on your system (`tr`, `sed`, etc.).

## Testing

### Run All Tests

```bash
# From safeclaw directory
go test ./...
```

### Run Tests with Verbose Output

```bash
go test -v ./...
```

### Run Tests with Race Detection

```bash
go test -race ./...
```

### Run Specific Test

```bash
go test -v -run TestJSONPlugin
```

### Run Tests with Coverage

```bash
go test -cover ./...
```

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Expected Test Output

All tests should pass:
```
ok      github.com/cadence-workflow/starlark-worker/safeclaw    0.677s
?       github.com/cadence-workflow/starlark-worker/safeclaw/examples/01_hello  [no test files]
...
?       github.com/cadence-workflow/starlark-worker/safeclaw/star       [no test files]
```

**Test Coverage**: 14 tests covering all plugins and core functionality

## Use Cases

- **AI Agent Tools**: Safe execution environment for agent-generated code
- **Configuration Scripts**: Dynamic configuration with Starlark
- **Data Processing**: ETL pipelines with Python-like syntax
- **Automation**: Shell-like scripting in Go applications
- **Sandboxed Execution**: Run untrusted code safely

## Dependencies

- `go.starlark.net` - Starlark interpreter
- `golang.org/x/crypto` - Blake2b hashing
- `golang.org/x/sync` - errgroup for concurrency
- `github.com/bitfield/script` - Shell-like operations
- `github.com/google/uuid` - UUID generation

All other functionality uses Go 1.25 stdlib.

## Quick Reference

### Common Operations

```go
// Create runner with custom logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
runner := safeclaw.NewRunner(plugin.DefaultPlugins(), logger)

// Run with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result, err := runner.RunSource(ctx, source, "function")

// Run with cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(1 * time.Second)
    cancel() // Cancel after 1 second
}()
result, err := runner.RunSource(ctx, source, "long_running_function")

// Pass multiple arguments
result, err := runner.RunSource(ctx, source, "function", "arg1", 42, true)

// Custom plugins
customPlugins := append(plugin.DefaultPlugins(), &MyPlugin{})
runner := safeclaw.NewRunner(customPlugins, nil)
```

### Starlark Script Patterns

```python
# Load multiple plugins
load("@plugin", "json", "request", "time")

# Error handling
def safe_operation():
    try:
        result = risky_function()
        return {"success": True, "result": result}
    except Exception as e:
        return {"success": False, "error": str(e)}

# List comprehensions
numbers = [i * 2 for i in range(10) if i % 2 == 0]

# Dictionary operations
config = {"key": "value"}
value = config.get("key", "default")

# Assertions (test plugin)
load("@plugin", t = "test")
t.equal(expected=5, actual=result)
t.true(value > 0)
```

### Performance Tips

1. **Reuse Runner**: Create once, use many times
2. **Use Context Timeouts**: Prevent runaway scripts
3. **Minimize Plugin Count**: Only load plugins you need
4. **Cache Compiled Scripts**: For repeated execution (requires custom implementation)
5. **Use Concurrent Plugin**: For parallel operations

### Security Considerations

- ✅ **Sandboxed**: Scripts can't access filesystem or network directly
- ✅ **Plugin-Controlled**: All powerful operations require explicit plugins
- ✅ **Timeout Support**: Context cancellation prevents infinite loops
- ✅ **No Eval**: Starlark doesn't have eval() or exec()
- ⚠️ **Resource Limits**: No built-in memory/CPU limits (use OS-level controls)
- ⚠️ **Plugin Security**: Plugins have full Go access - review custom plugins carefully

### Debugging

```go
// Enable debug logging
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
runner := safeclaw.NewRunner(plugin.DefaultPlugins(), logger)

// Errors include Starlark backtraces
result, err := runner.RunSource(ctx, source, "function")
if err != nil {
    // Error message includes full Starlark stack trace
    log.Printf("Execution failed:\n%v", err)
}
```

## Documentation

- **[AGENTS.md](AGENTS.md)** - Comprehensive implementation guide for future agents
- **[QUICKSTART.md](QUICKSTART.md)** - Quick start guide
- **[IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)** - Detailed implementation notes
- **[examples/](examples/)** - 10 complete, runnable examples

## Contributing

Contributions welcome! Areas for improvement:

- Additional plugins (database, messaging, etc.)
- Performance optimizations
- More examples
- Documentation improvements
- Bug fixes

## License

Same as parent project (starlark-worker).
