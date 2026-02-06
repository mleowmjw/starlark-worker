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

	// Simple Starlark script
	source := []byte(`
def greet(name, greeting="Hello"):
    return greeting + ", " + name + "!"
`)

	// Execute the function
	result, err := runner.RunSource(context.Background(), source, "greet", "World")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
