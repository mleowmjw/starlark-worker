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

// This program demonstrates Bug 2: HTTP Response Body Can Only Be Read Once
//
// Without caching, accessing response.text, response.json, or response.content
// multiple times fails because the body stream is exhausted after first read.

func main() {
	fmt.Println("=== Bug 2 Reproduction: Response Body Multiple Reads ===")
	fmt.Println("Testing multiple accesses to response body properties...")
	fmt.Println()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "test data from server", "count": 42, "active": true}`)
	}))
	defer server.Close()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "request", "json")

def test_multiple_reads(url):
    print("Making HTTP request...")
    response = request.do(method="GET", url=url)
    print("Request completed, status:", response.status_code)
    print()
    
    # First access: response.text
    print("First access: response.text")
    text1 = response.text
    print("  Length:", len(text1))
    print("  Content:", text1[:50] if len(text1) > 50 else text1)
    print()
    
    # Second access: response.text again (CRITICAL TEST)
    print("Second access: response.text (testing if body is cached)")
    text2 = response.text
    print("  Length:", len(text2))
    print("  Equal to first?", text1 == text2)
    print()
    
    # Third access: response.json (already parsed as dict)
    print("Third access: response.json (already parsed)")
    data = response.json  # This is already a parsed dict, not a string!
    print("  Message:", data["message"])
    print()
    
    # Fourth access: response.content (as bytes)
    print("Fourth access: response.content (as bytes)")
    content = response.content
    print("  Length:", len(content))
    print()
    
    return {
        "text1_length": len(text1),
        "text2_length": len(text2),
        "texts_equal": text1 == text2,
        "json_message": data["message"],
        "content_length": len(content),
        "all_same_length": len(text1) == len(text2) and len(text1) == len(content)
    }
`)

	result, err := runner.RunSource(context.Background(), source, "test_multiple_reads", server.URL)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	if err != nil {
		fmt.Printf("❌ Execution failed: %v\n", err)
		fmt.Println()
		fmt.Println("Bug 2 detected!")
		fmt.Println("The error indicates that reading the response body multiple times failed.")
		return
	}

	fmt.Printf("Final Result: %s\n", result)
	fmt.Println()

	resultStr := result.String()

	// Analyze results
	allGood := true

	if !strings.Contains(resultStr, `"texts_equal": True`) {
		fmt.Println("❌ Bug 2 detected: text1 != text2")
		fmt.Println("   Body content changed between reads (stream exhausted)")
		allGood = false
	} else {
		fmt.Println("✅ text1 == text2: Body cache is working")
	}

	if strings.Contains(resultStr, `"text2_length": 0`) {
		fmt.Println("❌ Bug 2 detected: Second read returned 0 bytes")
		fmt.Println("   The body stream was exhausted after first read")
		allGood = false
	} else {
		fmt.Println("✅ text2 has correct length: Multiple reads working")
	}

	if !strings.Contains(resultStr, `"json_message": "test data from server"`) {
		fmt.Println("❌ Bug 2 detected: JSON parsing failed or returned wrong data")
		allGood = false
	} else {
		fmt.Println("✅ JSON parsed correctly from cached body")
	}

	if !strings.Contains(resultStr, `"all_same_length": True`) {
		fmt.Println("❌ Bug 2 detected: Different lengths from different accesses")
		allGood = false
	} else {
		fmt.Println("✅ All accesses returned same length data")
	}

	fmt.Println()
	if allGood {
		fmt.Println("Result: ✅ NO BODY READ FAILURE (Bug 2 is FIXED)")
		fmt.Println()
		fmt.Println("Explanation:")
		fmt.Println("- The Response struct now has bodyCache and bodyRead fields")
		fmt.Println("- First call to readBody() reads and caches the body")
		fmt.Println("- Subsequent calls return the cached data")
		fmt.Println("- This allows multiple accesses to .text, .json, and .content")
	} else {
		fmt.Println("Result: ❌ Bug 2 exists - body caching not working")
	}
}
