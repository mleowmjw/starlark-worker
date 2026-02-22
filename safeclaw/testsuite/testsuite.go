package testsuite

import (
	"testing"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/stretchr/testify/require"
	temptestsuite "go.temporal.io/sdk/testsuite"
	temp "go.temporal.io/sdk/workflow"
)

// WorkflowTestSuite provides Temporal test environment for safeclaw scripts.
type WorkflowTestSuite struct {
	temptestsuite.WorkflowTestSuite
}

// WorkflowTestEnvironment wraps Temporal's test environment for safeclaw.
type WorkflowTestEnvironment struct {
	env    *temptestsuite.TestWorkflowEnvironment
	runner *safeclaw.Runner
}

// NewTestEnvironment creates a new test environment for safeclaw with Temporal.
func (s *WorkflowTestSuite) NewTestEnvironment(t *testing.T, plugins []safeclaw.Plugin) *WorkflowTestEnvironment {
	env := s.NewTestWorkflowEnvironment()
	
	// Create safeclaw runner
	runner := safeclaw.NewRunner(plugins, nil)
	
	return &WorkflowTestEnvironment{
		env:    env,
		runner: runner,
	}
}

// ExecuteScript executes a safeclaw script as a Temporal workflow.
func (e *WorkflowTestEnvironment) ExecuteScript(source []byte, function string, args ...interface{}) {
	workflowFunc := func(ctx temp.Context) (string, error) {
		// The runner will detect the Temporal context and set up the backend automatically
		// Pass ctx as interface{} to satisfy the new signature
		result, err := e.runner.RunSource(ctx, source, function, args...)
		if err != nil {
			return "", err
		}
		// Serialize starlark.Value to string for Temporal
		return result.String(), nil
	}
	
	e.env.ExecuteWorkflow(workflowFunc)
}

// GetResult retrieves the result of the workflow execution.
// The result is returned as a string representation of the Starlark value.
func (e *WorkflowTestEnvironment) GetResult(t *testing.T) string {
	require.True(t, e.env.IsWorkflowCompleted())
	require.NoError(t, e.env.GetWorkflowError())
	
	var result string
	require.NoError(t, e.env.GetWorkflowResult(&result))
	return result
}

// OnActivity mocks an activity for testing.
func (e *WorkflowTestEnvironment) OnActivity(activity interface{}, args ...interface{}) *temptestsuite.MockCallWrapper {
	return e.env.OnActivity(activity, args...)
}

// AssertExpectations verifies all expected activity calls were made.
func (e *WorkflowTestEnvironment) AssertExpectations(t *testing.T) {
	e.env.AssertExpectations(t)
}

// RegisterActivity registers an activity with the test environment.
func (e *WorkflowTestEnvironment) RegisterActivity(activity interface{}) {
	e.env.RegisterActivity(activity)
}

// SetTestTimeout sets the test timeout duration.
func (e *WorkflowTestEnvironment) SetTestTimeout(timeout time.Duration) {
	e.env.SetTestTimeout(timeout)
}

// RegisterDefaultActivities registers all common safeclaw activities.
func (e *WorkflowTestEnvironment) RegisterDefaultActivities() {
	// Import activities from plugins
	// These would be imported from respective plugin packages
	// Example: e.RegisterActivity(request.HTTPRequestActivity)
	// Example: e.RegisterActivity(sqlite.SQLExecActivity)
	// Example: e.RegisterActivity(script.ScriptExecActivity)
}
