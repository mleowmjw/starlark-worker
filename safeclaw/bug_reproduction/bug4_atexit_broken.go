package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

// This program demonstrates Bug 4: Atexit Plugin Non-Functional
//
// The atexit.register() and atexit.unregister() functions try to retrieve
// the module from thread-local storage, but the Runner never stores it there,
// causing "atexit module not found in thread" error.

func main() {
	fmt.Println("=== Bug 4 Reproduction: Atexit Plugin ===")
	fmt.Println("Testing atexit.register() functionality...")
	fmt.Println()

	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "atexit")

def cleanup():
    print("Cleanup function called!")
    return "cleaned up"

def test_atexit():
    print("Attempting to register exit hook...")
    atexit.register(cleanup)
    print("Successfully registered exit hook")
    return "registered"
`)

	result, err := runner.RunSource(context.Background(), source, "test_atexit")

	fmt.Println()

	if err != nil {
		if strings.Contains(err.Error(), "atexit module not found in thread") {
			fmt.Println("❌ Bug 4 detected: atexit module not found in thread")
			fmt.Println()
			fmt.Println("Error:", err)
			fmt.Println()
			fmt.Println("Root cause:")
			fmt.Println("- The atexit plugin's register() function calls t.Local(\"atexit_module\")")
			fmt.Println("- But the Runner never stores the module in thread-local storage")
			fmt.Println("- This makes the entire atexit plugin non-functional")
			fmt.Println()
			fmt.Println("Fix: In safeclaw.go, add after creating thread:")
			fmt.Println("  if atexitModule, ok := pluginModules[\"atexit\"]; ok {")
			fmt.Println("      thread.SetLocal(\"atexit_module\", atexitModule)")
			fmt.Println("  }")
			return
		}
		fmt.Printf("❌ Unexpected error: %v\n", err)
		return
	}

	fmt.Printf("Result: %s\n", result)
	fmt.Println()
	fmt.Println("✅ Atexit registration completed successfully")
	fmt.Println()
	fmt.Println("Result: ✅ ATEXIT PLUGIN FUNCTIONAL (Bug 4 is FIXED)")
	fmt.Println()
	fmt.Println("How this proves the bug is fixed:")
	fmt.Println("- The atexit.register() function needs to access the module instance")
	fmt.Println("- It retrieves it from thread-local storage using key 'atexit_module'")
	fmt.Println("- The Runner now stores the module during initialization")
	fmt.Println("- Registration succeeds without 'module not found' error")
}
