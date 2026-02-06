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
load("@plugin", "script")

def shell_operations():
    """Demonstrate shell-like operations using the script plugin"""
    
    # Echo and transform text
    result1 = script.echo("hello world").exec("tr a-z A-Z").string()
    
    # Count lines
    text = """line 1
line 2
line 3"""
    lines = script.echo(text).lines()
    
    # Pattern matching (simulate)
    result2 = script.echo("foo bar baz").exec("sed 's/bar/BAR/'").string()
    
    return {
        "uppercase": result1,
        "line_count": len(lines),
        "replaced": result2
    }
`)

	result, err := runner.RunSource(context.Background(), source, "shell_operations")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
