package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"go.starlark.net/starlark"
)

// ToolSpec defines a tool that an agent can execute
type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Script      string         `json:"script"`
	Function    string         `json:"function"`
	Args        map[string]any `json:"args"`
}

// ToolResult is the structured output from tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AgentToolExecutor runs tools in a sandboxed Starlark environment
type AgentToolExecutor struct {
	runner *safeclaw.Runner
}

func NewAgentToolExecutor() *AgentToolExecutor {
	return &AgentToolExecutor{
		runner: safeclaw.NewRunner(plugin.DefaultPlugins(), nil),
	}
}

func (e *AgentToolExecutor) Execute(ctx context.Context, spec ToolSpec) ToolResult {
	// Convert args to Starlark-compatible format
	var args []any
	for _, v := range spec.Args {
		args = append(args, v)
	}

	// Execute the tool script
	result, err := e.runner.RunSource(ctx, []byte(spec.Script), spec.Function, args...)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Convert result back to Go
	var goResult any
	if str, ok := result.(starlark.String); ok {
		goResult = string(str)
	} else {
		// For complex types, use the string representation
		goResult = result.String()
	}

	return ToolResult{
		Success: true,
		Result:  goResult,
	}
}

func main() {
	executor := NewAgentToolExecutor()

	// Define a tool for web scraping
	tool := ToolSpec{
		Name:        "fetch_and_parse",
		Description: "Fetch a URL and extract specific data",
		Script: `
load("@plugin", "request", "json")

def fetch_and_parse(url):
    """Fetch URL and parse JSON response"""
    response = request.do(method="GET", url=url)
    
    if response.status_code != 200:
        return {"error": "HTTP " + str(response.status_code)}
    
    data = json.loads(response.text)
    return {
        "status": "success",
        "data": data
    }
`,
		Function: "fetch_and_parse",
		Args: map[string]any{
			"url": "https://api.github.com/repos/google/starlark-go",
		},
	}

	// Execute the tool
	result := executor.Execute(context.Background(), tool)

	// Display result
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Tool Execution Result:\n%s\n", resultJSON)

	fmt.Println("\n=== Agent Tool Executor Demo ===")
	fmt.Println("This example demonstrates using Starlark as a safe, hermetic")
	fmt.Println("execution environment for agent tools. Key features:")
	fmt.Println("- Sandboxed execution (no direct filesystem/network access)")
	fmt.Println("- Controlled capabilities via plugins")
	fmt.Println("- Structured input/output")
	fmt.Println("- Safe for untrusted code execution")
}
