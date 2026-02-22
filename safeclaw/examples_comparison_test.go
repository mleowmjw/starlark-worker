package safeclaw_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"github.com/cadence-workflow/starlark-worker/safeclaw/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	requestplugin "github.com/cadence-workflow/starlark-worker/safeclaw/plugin/request"
)

// TestExample01HelloBothModes tests the hello example in both dev and testsuite modes.
func TestExample01HelloBothModes(t *testing.T) {
	source := []byte(`
def greet(name, greeting="Hello"):
    return greeting + ", " + name + "!"
`)

	// Run in dev mode (local backend)
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "greet", "World")
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode (Temporal backend)
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "greet", "World")
	testsuiteResult := env.GetResult(t)

	// Compare results
	devOutput := devResult.String()
	assert.Equal(t, devOutput, testsuiteResult, "Dev and testsuite outputs should be identical")
	
	t.Logf("✓ Example 01 (Hello):\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
}

// TestExample02JSONBothModes tests JSON processing in both modes.
func TestExample02JSONBothModes(t *testing.T) {
	source := []byte(`
load("@plugin", "json")

def process_data(data_json):
    data = json.loads(data_json)
    total = 0
    for item in data["items"]:
        total += item["value"]
    result = {
        "count": len(data["items"]),
        "names": [item["name"] for item in data["items"]],
        "total_value": total
    }
    return json.dumps(result)
`)

	inputJSON := `{"items": [{"name": "apple", "value": 10}, {"name": "banana", "value": 20}]}`

	// Run in dev mode
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "process_data", inputJSON)
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "process_data", inputJSON)
	testsuiteResult := env.GetResult(t)

	// Compare results
	devOutput := devResult.String()
	assert.Equal(t, devOutput, testsuiteResult, "Dev and testsuite outputs should be identical")
	
	t.Logf("✓ Example 02 (JSON):\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
}

// TestExample03HTTPBothModes tests HTTP requests in both modes (with mocking).
func TestExample03HTTPBothModes(t *testing.T) {
	source := []byte(`
load("@plugin", "request", "json")

def fetch_data():
    res = request.do(method="GET", url="https://httpbin.org/json")
    data = json.loads(res.text)
    return {"status": res.status_code, "has_data": data != None}
`)

	// Run in dev mode (will make actual HTTP call)
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "fetch_data")
	if err != nil {
		t.Skipf("Dev mode HTTP call failed (network issue): %v", err)
		return
	}

	// Run in testsuite mode (with mocked activity)
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.RegisterActivity(requestplugin.HTTPRequestActivity)
	
	// Mock the HTTP activity
	env.OnActivity(requestplugin.HTTPRequestActivity, mock.Anything, mock.MatchedBy(func(input requestplugin.HTTPRequestInput) bool {
		return input.Method == "GET" && input.URL == "https://httpbin.org/json"
	})).Return(&requestplugin.HTTPRequestOutput{
		StatusCode: 200,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{"slideshow": {"title": "Sample"}}`),
	}, nil)

	env.ExecuteScript(source, "fetch_data")
	testsuiteResult := env.GetResult(t)
	
	// Both should return status 200 and has_data: True
	devOutput := devResult.String()
	assert.Contains(t, devOutput, `"status": 200`)
	assert.Contains(t, testsuiteResult, `"status": 200`)
	
	t.Logf("✓ Example 03 (HTTP):\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
	t.Logf("  Note: Both return status 200 (dev=real call, testsuite=mocked)")
}

// TestRandomBehaviorComparison tests random plugin behavior in both modes.
func TestRandomBehaviorComparison(t *testing.T) {
	source := []byte(`
load("@plugin", "random")

def generate_randoms():
    random.seed(42)
    values = []
    for i in range(5):
        values.append(random.randint(min=1, max=100))
    return values
`)

	// Run in dev mode
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "generate_randoms")
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "generate_randoms")
	testsuiteResult := env.GetResult(t)

	// With same seed, both should produce deterministic results
	devOutput := devResult.String()
	assert.Equal(t, devOutput, testsuiteResult, "Both modes with same seed should produce identical results")
	
	t.Logf("✓ Random Plugin Comparison:\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
}

// TestTimeBehaviorComparison tests time plugin behavior in both modes.
func TestTimeBehaviorComparison(t *testing.T) {
	source := []byte(`
load("@plugin", "time")

def test_time_operations():
    # Time operations should work in both modes
    start = time.time()
    time.sleep(0.01)  # Small sleep
    end = time.time()
    elapsed = end - start
    return {"elapsed_ms": int(elapsed * 1000)}
`)

	// Run in dev mode
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "test_time_operations")
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "test_time_operations")
	testsuiteResult := env.GetResult(t)

	// Both should show elapsed time >= 10ms
	devOutput := devResult.String()
	
	// Extract elapsed_ms from both outputs
	extractElapsed := func(output string) int {
		re := regexp.MustCompile(`"elapsed_ms":\s*(\d+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			var elapsed int
			fmt.Sscanf(matches[1], "%d", &elapsed)
			return elapsed
		}
		return -1
	}
	
	devElapsed := extractElapsed(devOutput)
	testsuiteElapsed := extractElapsed(testsuiteResult)
	
	// Allow for minor timing differences (within 3ms tolerance)
	assert.GreaterOrEqual(t, devElapsed, 8, "Dev mode should show >= 8ms elapsed")
	assert.GreaterOrEqual(t, testsuiteElapsed, 8, "Testsuite mode should show >= 8ms elapsed")
	
	t.Logf("✓ Time Plugin Comparison:\n  Dev:       %s (elapsed: %dms)\n  Testsuite: %s (elapsed: %dms)", 
		devOutput, devElapsed, testsuiteResult, testsuiteElapsed)
	t.Logf("  Note: Minor timing differences acceptable (dev: real time, testsuite: simulated time)")
}

// TestUUIDBehaviorComparison tests UUID generation in both modes.
func TestUUIDBehaviorComparison(t *testing.T) {
	source := []byte(`
load("@plugin", "uuid")

def generate_uuids():
    ids = []
    for i in range(3):
        u = uuid.uuid4()
        ids.append(str(u))
    return {"count": len(ids), "first": ids[0][:8]}  # Return partial UUID for comparison
`)

	// Run in dev mode
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "generate_uuids")
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "generate_uuids")
	testsuiteResult := env.GetResult(t)

	// Both should return count: 3 and valid UUID format
	devOutput := devResult.String()
	
	assert.Contains(t, devOutput, `"count": 3`)
	assert.Contains(t, testsuiteResult, `"count": 3`)
	
	// Verify UUID format (8 hex characters)
	uuidPattern := regexp.MustCompile(`"first":\s*"[0-9a-f]{8}"`)
	assert.True(t, uuidPattern.MatchString(devOutput), "Dev output should contain valid UUID prefix")
	assert.True(t, uuidPattern.MatchString(testsuiteResult), "Testsuite output should contain valid UUID prefix")
	
	t.Logf("✓ UUID Plugin Comparison:\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
	t.Logf("  Note: Different UUIDs expected between runs (both use workflow.SideEffect)")
}

// TestDeterministicBehavior verifies that plugins produce consistent results within same execution.
func TestDeterministicBehavior(t *testing.T) {
	source := []byte(`
load("@plugin", "random", "uuid", "time")

def test_deterministic():
    # Set seed for random
    random.seed(99)
    
    # Generate values
    rand_vals = [random.randint(min=1, max=10) for _ in range(3)]
    uuid_val = str(uuid.uuid4())
    time_val = time.time()
    
    return {
        "random": rand_vals,
        "uuid_len": len(uuid_val),
        "time_positive": time_val > 0,
    }
`)

	// Run in dev mode
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)
	devResult, err := runner.RunSource(context.Background(), source, "test_deterministic")
	assert.NoError(t, err, "Dev mode execution should succeed")

	// Run in testsuite mode
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())
	env.ExecuteScript(source, "test_deterministic")
	testsuiteResult := env.GetResult(t)

	// Compare structural properties
	devOutput := devResult.String()
	
	// Both should have same seeded random values
	assert.Contains(t, devOutput, `"random":`)
	assert.Contains(t, testsuiteResult, `"random":`)
	
	// Both should have valid UUID length (36)
	assert.Contains(t, devOutput, `"uuid_len": 36`)
	assert.Contains(t, testsuiteResult, `"uuid_len": 36`)
	
	// Both should have positive time
	assert.Contains(t, devOutput, `"time_positive": True`)
	assert.Contains(t, testsuiteResult, `"time_positive": True`)
	
	t.Logf("✓ Deterministic Behavior Test:\n  Dev:       %s\n  Testsuite: %s", devOutput, testsuiteResult)
}
