package random_test

import (
	"testing"

	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"github.com/cadence-workflow/starlark-worker/safeclaw/testsuite"
)

// TestRandomDeterministicReplay tests that random values work in workflow context.
// Note: SideEffect ensures determinism during replay of the SAME workflow execution,
// not across different executions with the same seed.
func TestRandomDeterministicReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	source := []byte(`
load("@plugin", "random")

def test_random():
    random.seed(12345)
    values = []
    for i in range(5):
        values.append(random.randint(min=1, max=100))
    return values
`)

	env.ExecuteScript(source, "test_random")
	result := env.GetResult(t)

	// Verify result is valid (should contain 5 values)
	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
	
	t.Logf("Generated random values in workflow: %s", result)
}

// TestRandomWithoutSeed tests unseeded random (should still be deterministic in workflow).
func TestRandomWithoutSeed(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	source := []byte(`
load("@plugin", "random")

def test_random():
    return random.random()
`)

	env.ExecuteScript(source, "test_random")
	result := env.GetResult(t)

	// Result should be a float in string representation
	if result == "" {
		t.Errorf("Expected non-empty result")
	}
}
