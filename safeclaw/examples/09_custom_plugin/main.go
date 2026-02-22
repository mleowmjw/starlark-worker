package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

// CustomPlugin demonstrates how to create a custom plugin
type CustomPlugin struct{}

func (p *CustomPlugin) ID() string {
	return "custom"
}

func (p *CustomPlugin) Module(ctx interface{}, info safeclaw.RunInfo) starlark.Value {
	return &CustomModule{}
}

// CustomModule is the Starlark module implementation
type CustomModule struct{}

var _ starlark.HasAttrs = &CustomModule{}

func (m *CustomModule) String() string        { return "custom" }
func (m *CustomModule) Type() string          { return "custom" }
func (m *CustomModule) Freeze()                {}
func (m *CustomModule) Truth() starlark.Bool  { return true }
func (m *CustomModule) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: custom") }

func (m *CustomModule) Attr(n string) (starlark.Value, error) {
	return star.Attr(m, n, builtins, properties)
}

func (m *CustomModule) AttrNames() []string {
	return star.AttrNames(builtins, properties)
}

var builtins = map[string]*starlark.Builtin{
	"greet":    starlark.NewBuiltin("greet", greet),
	"multiply": starlark.NewBuiltin("multiply", multiply),
}

var properties = map[string]star.PropertyFactory{
	"version": func(receiver starlark.Value) (starlark.Value, error) {
		return starlark.String("1.0.0"), nil
	},
}

func greet(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	if err := starlark.UnpackArgs("greet", args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	return starlark.String(fmt.Sprintf("Hello, %s, from custom plugin!", name)), nil
}

func multiply(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var a, b int
	if err := starlark.UnpackArgs("multiply", args, kwargs, "a", &a, "b", &b); err != nil {
		return nil, err
	}
	return starlark.MakeInt(a * b), nil
}

func main() {
	// Create plugins including our custom one
	plugins := append(plugin.DefaultPlugins(), &CustomPlugin{})
	runner := safeclaw.NewRunner(plugins, nil)

	source := []byte(`
load("@plugin", "custom")

def use_custom_plugin():
    greeting = custom.greet(name="World")
    product = custom.multiply(a=6, b=7)
    version = custom.version
    
    return {
        "greeting": greeting,
        "product": product,
        "version": version
    }
`)

	result, err := runner.RunSource(context.Background(), source, "use_custom_plugin")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)
}
