# Safeclaw Quick Start Guide

## Installation

```bash
cd safeclaw
go mod tidy
```

## 30-Second Demo

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
    runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

    script := []byte(`
load("@plugin", "json", "time")

def process(name):
    return {
        "greeting": "Hello, " + name,
        "timestamp": time.time()
    }
`)

    result, err := runner.RunSource(context.Background(), script, "process", "World")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result)
}
```

## Run Examples

```bash
# Example 1: Hello World
cd examples/01_hello && go run main.go

# Example 2: JSON Processing
cd examples/02_json && go run main.go

# Example 3: HTTP Client
cd examples/03_http && go run main.go

# Example 4: Concurrent Tasks
cd examples/04_concurrent && go run main.go

# Example 10: Agent Tool Executor
cd examples/10_agent_tool && go run main.go
```

## Run Tests

```bash
go test ./...
```

## Common Patterns

### 1. Simple Function Call

```go
source := []byte(`def add(a, b): return a + b`)
result, _ := runner.RunSource(ctx, source, "add", 2, 3)
fmt.Println(result) // 5
```

### 2. Using Plugins

```go
source := []byte(`
load("@plugin", "json", "hashlib")

def hash_data(data):
    json_str = json.dumps(data)
    return hashlib.blake2b_hex(data=json_str, digest_size=16)
`)

result, _ := runner.RunSource(ctx, source, "hash_data", map[string]interface{}{
    "name": "test",
    "value": 42,
})
```

### 3. Concurrent Execution

```go
source := []byte(`
load("@plugin", "concurrent")

def worker(n):
    return n * n

def parallel_squares(count):
    futures = [concurrent.run(worker, i) for i in range(count)]
    return [f.result() for f in futures]
`)

result, _ := runner.RunSource(ctx, source, "parallel_squares", 10)
```

### 4. HTTP Requests

```go
source := []byte(`
load("@plugin", "request", "json")

def fetch_api(url):
    response = request.do(method="GET", url=url)
    return json.loads(response.text)
`)

result, _ := runner.RunSource(ctx, source, "fetch_api", "https://api.example.com/data")
```

### 5. Custom Plugin

```go
type MyPlugin struct{}

func (p *MyPlugin) ID() string { return "myplugin" }

func (p *MyPlugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
    return &MyModule{} // Implement starlark.HasAttrs
}

// Use it
plugins := append(plugin.DefaultPlugins(), &MyPlugin{})
runner := safeclaw.NewRunner(plugins, nil)
```

## Plugin Reference

| Plugin | Import | Key Functions |
|--------|--------|---------------|
| json | `load("@plugin", "json")` | `dumps()`, `loads()` |
| time | `load("@plugin", "time")` | `time()`, `sleep()`, `time_ns()` |
| random | `load("@plugin", "random")` | `seed()`, `randint()`, `random()` |
| uuid | `load("@plugin", "uuid")` | `uuid4()` |
| os | `load("@plugin", "os")` | `os.environ` |
| hashlib | `load("@plugin", "hashlib")` | `blake2b_hex()` |
| test | `load("@plugin", t="test")` | `t.true()`, `t.equal()` |
| concurrent | `load("@plugin", "concurrent")` | `run()`, `batch_run()` |
| request | `load("@plugin", "request")` | `do()` |
| script | `load("@plugin", "script")` | `exec()`, `file()`, `echo()` |
| progress | `load("@plugin", "progress")` | `report()` |
| atexit | `load("@plugin", "atexit")` | `register()`, `unregister()` |

## Built-in Types

### Dataclass
```python
Person = Dataclass(name="", age=0)
p = Person(name="Alice", age=30)
print(p.name)  # Alice
```

### CallableObject
```python
def greet(name):
    return "Hello, " + name

obj = CallableObject(greet)
obj.version = "1.0"
print(obj("World"))  # Hello, World
print(obj.version)   # 1.0
```

## Error Handling

```go
result, err := runner.RunSource(ctx, source, "function", args...)
if err != nil {
    var evalErr *starlark.EvalError
    if errors.As(err, &evalErr) {
        fmt.Println("Starlark error:", evalErr.Backtrace())
    }
    return err
}
```

## Best Practices

1. **Use Context**: Always pass a proper context for cancellation
2. **Log Errors**: Enable slog for debugging
3. **Test Scripts**: Use the `test` plugin for assertions
4. **Sandbox**: Safeclaw is hermetic - only plugins can access external resources
5. **Performance**: Reuse Runner instances, they're safe for concurrent use

## Troubleshooting

### Import Errors
```
Error: module not found
```
**Solution**: Use `load("@plugin", "name")` not `import name`

### Network Errors
```
Error: request failed
```
**Solution**: Ensure the `request` plugin is loaded and context is valid

### Type Errors
```
Error: bad argument type
```
**Solution**: Use `json.dumps()` to convert complex Go types before passing

## Next Steps

- Read the [README.md](README.md) for full documentation
- Explore [examples/](examples/) for more patterns
- Check [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for architecture details
- Create custom plugins for your use case

## Support

This is a standalone extraction from `starlark-worker`. For issues:
1. Check examples first
2. Review plugin source code
3. Test with minimal reproduction
4. File issue with starlark-worker project
