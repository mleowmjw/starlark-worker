package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

// This program demonstrates Bug 3: Context Cancellation Not Propagated
//
// When batchRun uses context.Background() instead of the actual context,
// timeouts and cancellations don't work, causing goroutines to run indefinitely.

func main() {
	fmt.Println("=== Bug 3 Reproduction: Context Cancellation ===")
	fmt.Println("Testing that context cancellation stops batch operations...")
	fmt.Println()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "concurrent", "time")

def slow_worker(n):
    print("Worker", n, "starting (will sleep 10 seconds)...")
    time.sleep(seconds=10.0)
    print("Worker", n, "completed")
    return "Task " + str(n)

def run_batch():
    # Create 3 callables that each take 10 seconds
    callables = []
    for i in range(3):
        c = concurrent.new_callable(slow_worker, i)
        callables.append(c)
    
    print("Starting batch of 3 workers (30 seconds total without cancellation)...")
    batch = concurrent.batch_run(callables, max_concurrency=3)
    results = batch.result()
    return results
`)

	// Create a context with 2 second timeout
	fmt.Println("Creating context with 2 second timeout...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	result, err := runner.RunSource(ctx, source, "run_batch")
	elapsed := time.Since(start)

	fmt.Println()
	fmt.Printf("Execution took: %v\n", elapsed)
	fmt.Println()

	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") || 
		   strings.Contains(err.Error(), "context canceled") {
			fmt.Println("✅ Context cancellation worked correctly")
			fmt.Printf("✅ Execution stopped after %v (expected ~2s)\n", elapsed)
			fmt.Println()
			fmt.Println("Result: ✅ CONTEXT CANCELLATION WORKS (Bug 3 is FIXED)")
			fmt.Println()
			fmt.Println("How this proves the bug is fixed:")
			fmt.Println("- Workers are set to run for 10 seconds each (30s total)")
			fmt.Println("- Context timeout is 2 seconds")
			fmt.Println("- Execution stopped at ~2s, proving cancellation propagated")
			fmt.Println("- With Bug 3 unfixed, it would run for 30+ seconds ignoring timeout")
		} else {
			fmt.Printf("❌ Unexpected error: %v\n", err)
		}
	} else {
		fmt.Println("❌ Bug 3 detected: Execution completed without timeout error")
		fmt.Printf("❌ Result: %s\n", result)
		fmt.Println()
		fmt.Println("This means context cancellation was NOT propagated!")
		fmt.Println("With Bug 3 unfixed, the batch would use context.Background()")
		fmt.Println("and ignore the 2-second timeout from the parent context.")
	}

	if elapsed > 5*time.Second {
		fmt.Printf("❌ Bug 3 detected: Execution took %v (should be ~2s)\n", elapsed)
		fmt.Println("Context cancellation failed - batch operations continued running")
	}
}
