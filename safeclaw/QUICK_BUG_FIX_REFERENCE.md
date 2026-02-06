# Quick Bug Fix Reference

A concise reference for the 4 critical bugs fixed in SafeClaw, for quick consultation.

---

## Bug 1: Resource Leak ✅

**Pattern**: Always defer Close() on resources
```go
res, err := client.Do(req)
if err != nil {
    return nil, err
}
defer res.Body.Close()  // ✅ Always close immediately
```

---

## Bug 2: Stream Exhaustion ✅

**Pattern**: Cache stream data on first read
```go
type Response struct {
    bodyCache []byte
    bodyRead  bool
}

func (r *Response) readBody() ([]byte, error) {
    if !r.bodyRead {
        r.bodyCache, _ = io.ReadAll(r.body)
        r.bodyRead = true
    }
    return r.bodyCache, nil  // ✅ Return cached data
}
```

---

## Bug 3: Context Cancellation ✅

**Pattern**: Use select with ctx.Done() for blocking operations
```go
func sleep(t *starlark.Thread, duration time.Duration) error {
    ctx := safeclaw.GetContext(t)
    timer := time.NewTimer(duration)
    defer timer.Stop()
    
    select {
    case <-timer.C:
        return nil
    case <-ctx.Done():
        return ctx.Err()  // ✅ Respect cancellation
    }
}
```

**For CPU-bound loops**:
```go
for i := 0; i < n; i++ {
    if i%100 == 0 {  // Check periodically
        if err := ctx.Err(); err != nil {
            return err  // ✅ Stop on cancellation
        }
    }
    // Work...
}
```

---

## Bug 4: Thread-Local Storage ✅

**Pattern**: Store stateful plugin modules in thread-local storage
```go
// In Runner.run():
if myModule, ok := pluginModules["myplugin"]; ok {
    thread.SetLocal("myplugin_module", myModule)  // ✅ Store module
}

// In plugin function:
func myFunc(t *starlark.Thread, ...) (starlark.Value, error) {
    moduleVal := t.Local("myplugin_module")
    if moduleVal == nil {
        return nil, fmt.Errorf("module not found")
    }
    module := moduleVal.(*Module)
    // Modify module.state
}
```

---

## Testing Checklist

When adding new plugins:

- [ ] Resources: All acquired resources are cleaned up
- [ ] Context: Long-running operations check `ctx.Done()`
- [ ] Storage: Stateful plugins stored in thread-local storage if needed
- [ ] Caching: Streamed data cached for multiple reads
- [ ] Errors: Clear, specific, actionable messages
- [ ] Race: `go test -race` passes
- [ ] Integration: Works with other plugins
- [ ] Examples: Demonstrates typical usage
- [ ] Docs: API behavior clearly explained

---

## Quick Diagnostics

**"Too many open files"**  
→ Check for missing `defer Close()` on HTTP responses, files, connections

**"Empty response body on second access"**  
→ Implement body caching (see Bug 2 pattern)

**"Timeout ignored, operation continues"**  
→ Add context checks in blocking operations (see Bug 3 pattern)

**"Module not found in thread"**  
→ Store module in thread-local storage (see Bug 4 pattern)

**Race condition detected**  
→ Check for shared state without mutex protection
→ Verify goroutine cleanup on cancellation

---

## Bug Reproduction Tests

Location: `safeclaw/bug_reproduction/`

Run all tests:
```bash
cd safeclaw/bug_reproduction
for f in bug*.go; do
    echo "=== Testing: $f ==="
    go run "$f" | tail -5
done
```

These tests serve as regression tests - run them after changes to plugins.

---

## References

- Full documentation: `AGENTS.md` → "Critical Bug Fixes & Learnings"
- Detailed analysis: `BUG_REPRODUCTION_RESULTS.md`
- Fix summary: `BUG_FIXES_SUMMARY.md`
- Proof of fixes: `BUG_FIXES_PROOF.md`
