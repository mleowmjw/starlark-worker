package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/ext"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

func main() {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	// Create a tar archive with multiple files
	files := map[string][]byte{
		"/main.star": []byte(`
load("config.json", "json")
load("data.txt", "txt")

def process():
    return {
        "config": json,
        "data": txt,
        "message": "Processed " + str(len(txt)) + " bytes of data"
    }
`),
		"/config.json": []byte(`{"version": "1.0", "enabled": true}`),
		"/data.txt":    []byte("Hello from a text file!"),
	}

	var tarBuf bytes.Buffer
	if err := ext.WriteTar(files, &tarBuf); err != nil {
		log.Fatalf("Failed to create tar: %v", err)
	}

	result, err := runner.RunTar(context.Background(), tarBuf.Bytes(), "/main.star", "process")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
