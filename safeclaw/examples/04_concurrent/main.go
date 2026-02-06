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

	source := []byte(`
load("@plugin", "concurrent", "time")

def worker(n):
    # Simulate some work
    time.sleep(seconds=0.1)
    return "Task " + str(n) + " completed"

def run_parallel(count):
    # Start multiple tasks concurrently
    futures = []
    for i in range(count):
        f = concurrent.run(worker, i)
        futures.append(f)
    
    # Collect results
    results = [f.result() for f in futures]
    return results
`)

	result, err := runner.RunSource(context.Background(), source, "run_parallel", 5)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
