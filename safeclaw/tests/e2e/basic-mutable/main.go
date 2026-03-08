package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/uuid"
)

// Start workflow by sending inout to script? to run script ..
func main() {
	runSanityCheck()
}

func runTestCheck() {
	// Tigger TEST mode; will use Temporal TestSuite ..
	// set env SAFECLAW_ENV = test ..
}

func runSanityCheck() {
	// Default env is DEV ..
	/*
		[]safeclaw.Plugin{
			atexit.Plugin,
			chameleon.Plugin,
			concurrent.Plugin,
			hashlib.Plugin,
			json.Plugin,
			os.Plugin,
			progress.Plugin,
			random.Plugin,
			request.Plugin,
			script.Plugin,
			sqlite.Plugin,
			test.Plugin,
			time.Plugin,
			uuid.Plugin,
			whatsapp.Plugin,
		}
	*/
	// Safer to only load the plugins needed explicitly .. lock down the
	// runner of each step .. there is probably a minimal used everywhere
	// related to the libs??
	runner := safeclaw.NewRunner([]safeclaw.Plugin{
		uuid.Plugin,
	}, nil)
	//runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	//spew.Dump(runner)
	// Simple Starlark script
	source := []byte(`
load("@plugin", "uuid")

def greet(name, greeting="Goodbye"):
	# res = uuid.uuid4().hex
	res = uuid.uuid4().bob
	return greeting + ", " + name + "  " + res + "!"
`)

	// Execute the function
	result, err := runner.RunSource(context.Background(), source, "greet", "World")
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result)

}
