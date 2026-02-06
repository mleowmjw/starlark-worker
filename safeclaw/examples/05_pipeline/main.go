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
load("@plugin", "json", "hashlib")

def extract(raw_data):
    """Stage 1: Extract data from JSON"""
    data = json.loads(raw_data)
    return data["records"]

def transform(records):
    """Stage 2: Transform records"""
    transformed = []
    for record in records:
        transformed.append({
            "id": record["id"],
            "name": record["name"].upper(),
            "hash": hashlib.blake2b_hex(data=record["name"], digest_size=16)
        })
    return transformed

def load_data(records):
    """Stage 3: Aggregate results"""
    return {
        "total": len(records),
        "items": records
    }

def pipeline(raw_data):
    """Run the full ETL pipeline"""
    records = extract(raw_data)
    transformed = transform(records)
    result = load_data(transformed)
    return json.dumps(result)
`)

	inputData := `{
		"records": [
			{"id": 1, "name": "alice"},
			{"id": 2, "name": "bob"},
			{"id": 3, "name": "charlie"}
		]
	}`

	result, err := runner.RunSource(context.Background(), source, "pipeline", inputData)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
