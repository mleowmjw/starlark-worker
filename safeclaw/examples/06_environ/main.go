package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

func main() {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "os", "time")

def get_config():
    """Read configuration from environment"""
    config = {
        "app_name": os.environ.get("APP_NAME", "default-app"),
        "debug": os.environ.get("DEBUG", "false"),
        "current_time": time.time(),
    }
    return config
`)

	// For this example, we demonstrate environment variable access
	// In a real scenario, you'd extend the Runner API to accept custom RunInfo
	// with environment variables
	_ = safeclaw.RunInfo{
		Environ: map[string]string{
			"APP_NAME":      "safeclaw-demo",
			"DEBUG":         "true",
			"STARLARK_TIME": fmt.Sprintf("unix:%d", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()),
		},
		StartTime: time.Now(),
		Logger:    nil,
	}

	ctx := context.Background()
	
	result, err := runner.RunSource(ctx, source, "get_config")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
	fmt.Println("\nNote: This example demonstrates environment variable access.")
	fmt.Println("The STARLARK_TIME env var allows backfilling scripts to a specific time.")
}
