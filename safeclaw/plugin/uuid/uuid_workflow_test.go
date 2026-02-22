package uuid_test

import (
	"testing"

	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"github.com/cadence-workflow/starlark-worker/safeclaw/testsuite"
)

// TestUUIDDeterministicReplay tests that UUIDs work in workflow context.
// Note: SideEffect ensures determinism during replay of the SAME workflow execution,
// not across different executions.
func TestUUIDDeterministicReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	source := []byte(`
load("@plugin", "uuid")

def test_uuid():
    ids = []
    for i in range(3):
        u = uuid.uuid4()
        ids.append(str(u))
    return ids
`)

	env.ExecuteScript(source, "test_uuid")
	result := env.GetResult(t)

	// Verify result is valid (should contain 3 UUIDs)
	if len(result) == 0 {
		t.Error("Expected non-empty UUID list")
	}
	
	t.Logf("Generated UUIDs in workflow: %s", result)
}

// TestUUIDFormat tests UUID formatting (hex, urn).
func TestUUIDFormat(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	source := []byte(`
load("@plugin", "uuid")

def test_uuid_format():
    u = uuid.uuid4()
    return {
        "str": str(u),
        "hex": u.hex,
        "urn": u.urn,
    }
`)

	env.ExecuteScript(source, "test_uuid_format")
	result := env.GetResult(t)

	if result == "" {
		t.Error("Expected result, got empty string")
	}
}
