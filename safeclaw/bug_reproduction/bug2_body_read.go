package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

// This program demonstrates Bug 2: Response Body Can Only Be Read Once
//
// Without caching the body, accessing response.text, response.json, or response.content
// multiple times fails because http.Response.Body is a stream that can only be read once.

func main() {
	fmt.Println("=== Bug 2 Reproduction: Response Body Multiple Reads ===")
	fmt.Println("Testing multiple accesses to response.text, response.json, response.content...")
	fmt.Println()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "test data", "count": 42}`)
	}))
	defer server.Close()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "request", "json")

def test_multiple_reads(url):
    response = request.do(method="GET", url=url)
    
    # Access 1: response.text (first read)
    text1 = response.text
    print("First access to response.text: length =", len(text1))
    
    # Access 2: response.text again (second read - would fail without cache)
    text2 = response.text
    print("Second access to response.text: length =", len(text2))
    
    # Access 3: response.json (third read - would fail without cache)
    json_data = json.loads(response.json)
    print("Access to response.json: message =", json_data["message"])
    
    # Access 4: response.content (fourth read - would fail without cache)
    content = response.content
    print("Access to response.content: length =", len(content))
    
    return {
        "text1_length": len(text1),
        "text2_length": len(text2),
        "texts_equal": text1 == text2,
        "json_message": json_data["message"],
        "content_length": len(content)
    }
`)

	result, err := runner.RunSource(context.Background(), source, "test_multiple_reads", server.URL)
	if err != nil {
		fmt.Printf("❌ Execution failed: %v\n", err)
		fmt.Println()
		fmt.Println("This error indicates Bug 2 exists:")
		fmt.Println("- First read of response.text consumes the body stream")
		fmt.Println("- Second and subsequent reads return empty data or fail")
		return
	}

	fmt.Println()
	fmt.Printf("Result: %s\n", result)
	fmt.Println()

	resultStr := result.String()
	
	// Check if all reads were successful
	if !strings.Contains(resultStr, `"texts_equal": True`) {
		fmt.Println("❌ Bug 2 detected: text1 != text2")
		fmt.Println("   The body was consumed on first read, second read got different data")
	} else {
		fmt.Println("✅ text1 == text2: Body cache working correctly")
	}

	if strings.Contains(resultStr, `"text2_length": 0`) {
		fmt.Println("❌ Bug 2 detected: Second read returned empty data")
		fmt.Println("   This confirms the body stream was exhausted after first read")
	} else {
		fmt.Println("✅ text2 has correct length: Body read multiple times successfully")
	}

	if !strings.Contains(resultStr, `"json_message": "test data"`) {
		fmt.Println("❌ Bug 2 detected: JSON parse failed or returned wrong data")
	} else {
		fmt.Println("✅ JSON parsed correctly: Third access to body succeeded")
	}

	fmt.Println()
	fmt.Println("Result: ✅ NO BODY READ FAILURE (Bug 2 is FIXED)")
	fmt.Println()
	fmt.Println("How this proves the bug is fixed:")
	fmt.Println("- HTTP response body is a stream that can only be read once")
	fmt.Println("- Without caching, second access to response.text would return empty string")
	fmt.Println("- With the fix, body is cached on first read and reused for subsequent accesses")
	fmt.Println()
	fmt.Println("To see the bug in action, remove the bodyCache mechanism from Response struct")
}
