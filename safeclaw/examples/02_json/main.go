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
load("@plugin", "json")

def process_data(data_json):
    # Parse JSON
    data = json.loads(data_json)
    
    # Calculate total value (Starlark doesn't have sum())
    total = 0
    for item in data["items"]:
        total += item["value"]
    
    # Transform it
    result = {
        "count": len(data["items"]),
        "names": [item["name"] for item in data["items"]],
        "total_value": total
    }
    
    # Return as JSON string
    return json.dumps(result)
`)

	inputJSON := `{
		"items": [
			{"name": "apple", "value": 10},
			{"name": "banana", "value": 20},
			{"name": "cherry", "value": 30}
		]
	}`

	result, err := runner.RunSource(context.Background(), source, "process_data", inputJSON)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
