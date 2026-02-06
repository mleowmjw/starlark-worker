package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

func main() {
	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "Hello from server", "method": "%s"}`, r.Method)
	}))
	defer server.Close()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "request", "json")

def fetch_data(url):
    # Make HTTP GET request
    response = request.do(method="GET", url=url)
    
    # Parse JSON response
    data = json.loads(response.text)
    
    return {
        "status": response.status_code,
        "message": data["message"],
        "method": data["method"]
    }
`)

	result, err := runner.RunSource(context.Background(), source, "fetch_data", server.URL)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
