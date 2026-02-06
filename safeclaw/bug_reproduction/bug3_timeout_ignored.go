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
// When errgroup uses context.Background() instead of the execution context,
// batch operations ignore parent context cancellation and timeouts.

func main() {
	fmt.Println("=== Bug 3 Reproduction: Context Timeout in Batch Operations ===")
	fmt.Println()
	fmt.Println("Scenario:")
	fmt.Println("- 5 workers, each sleeping for 10 seconds (50 seconds total)")
	fmt.Println("- Context timeout set to 2 seconds")
	fmt.Println("- Expected: Execution stops after ~2 seconds with timeout error")
	fmt.Println("- Bug: With context.Background(), batch ignores timeout and runs for 50s")
	fmt.Println()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "concurrent", "time")

def slow_worker(n):
    # Each worker takes 10 seconds
    time.sleep(seconds=10.0)
    return "Task " + str(n)

def run_batch():
    # Create 5 callables that each take 10 seconds
    callables = []
    for i in range(5):
        c = concurrent.new_callable(slow_worker, i)
        callables.append(c)
    
    # Start batch (would take 50 seconds total without cancellation)
    batch = concurrent.batch_run(callables, max_concurrency=5)
    results = batch.result()
    return results
`)

	// Create a context with 2 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	fmt.Println("Starting batch execution with 2-second timeout...")
	fmt.Println()

	start := time.Now()
	result, err := runner.RunSource(ctx, source, "run_batch")
	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Execution completed in: %v\n", elapsed)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	hasTimeoutError := false
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "context deadline exceeded") || 
		   strings.Contains(errMsg, "context canceled") {
			hasTimeoutError = true
		}
		fmt.Printf("Error: %v\n", err)
		fmt.Println()
	} else {
		fmt.Printf("Result: %s\n", result)
		fmt.Println()
	}

	// Analyze results
	if !hasTimeoutError && err == nil {
		fmt.Println("❌ Bug 3 detected: NO TIMEOUT ERROR")
		fmt.Println("   Execution completed normally, ignoring context timeout")
		fmt.Println("   This proves context.Background() was used instead of actual context")
		fmt.Println()
		fmt.Println("Result: ❌ Bug 3 EXISTS - Context cancellation not working")
	} else if hasTimeoutError && elapsed < 5*time.Second {
		fmt.Println("✅ Timeout error received as expected")
		fmt.Printf("✅ Execution stopped quickly (%v), respecting 2s timeout\n", elapsed)
		fmt.Println()
		fmt.Println("Result: ✅ CONTEXT CANCELLATION WORKS (Bug 3 is FIXED)")
		fmt.Println()
		fmt.Println("How this proves the bug is fixed:")
		fmt.Println("- errgroup now uses the actual execution context, not context.Background()")
		fmt.Println("- When parent context times out, errgroup context is cancelled")
		fmt.Println("- Goroutines check gCtx.Err() and stop early")
		fmt.Println("- Execution stopped at ~2s instead of running for 50s")
	} else {
		fmt.Printf("⚠️  Ambiguous result: error=%v, elapsed=%v\n", err != nil, elapsed)
	}

	fmt.Println()
	fmt.Println("Performance comparison:")
	fmt.Println("- With Bug 3 (context.Background()): Would run for 50+ seconds")
	fmt.Printf("- With Fix (actual context): Stopped after %v\n", elapsed)
	fmt.Printf("- Time saved: ~%v\n", 50*time.Second-elapsed)
}
