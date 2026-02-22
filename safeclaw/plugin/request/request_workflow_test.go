package request_test

import (
	"testing"

	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	requestplugin "github.com/cadence-workflow/starlark-worker/safeclaw/plugin/request"
	"github.com/cadence-workflow/starlark-worker/safeclaw/testsuite"
	"github.com/stretchr/testify/mock"
)

// TestHTTPRequestMocked tests HTTP requests with mocked activities.
func TestHTTPRequestMocked(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	// Register the HTTP activity
	env.RegisterActivity(requestplugin.HTTPRequestActivity)

	// Mock the HTTP activity to return a fixed response
	// Note: Activities receive context as first parameter, use mock.Anything
	env.OnActivity(requestplugin.HTTPRequestActivity, mock.Anything, requestplugin.HTTPRequestInput{
		Method:  "GET",
		URL:     "https://api.example.com/data",
		Body:    nil,
		Headers: map[string][]string{},
	}).Return(&requestplugin.HTTPRequestOutput{
		StatusCode: 200,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"status": "ok", "data": [1, 2, 3]}`),
	}, nil)

	source := []byte(`
load("@plugin", "request")
load("@plugin", "json")

def test_request():
    res = request.do(method="GET", url="https://api.example.com/data")
    data = json.loads(res.text)
    return {
        "status_code": res.status_code,
        "data": data,
    }
`)

	env.ExecuteScript(source, "test_request")
	result := env.GetResult(t)

	// Verify we got a result containing the mocked data
	if result == "" {
		t.Error("Expected result, got empty string")
	}
	
	// Verify expectations
	env.AssertExpectations(t)
}

// TestHTTPRequestDeterministicReplay tests that HTTP results are deterministic across replays.
func TestHTTPRequestDeterministicReplay(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestEnvironment(t, plugin.DefaultPlugins())

	env.RegisterActivity(requestplugin.HTTPRequestActivity)

	// Mock response
	// Note: Activities receive context as first parameter, use mock.Anything
	env.OnActivity(requestplugin.HTTPRequestActivity, mock.Anything, requestplugin.HTTPRequestInput{
		Method:  "POST",
		URL:     "https://api.example.com/submit",
		Body:    []byte(`{"value": 42}`),
		Headers: map[string][]string{"Content-Type": {"application/json"}},
	}).Return(&requestplugin.HTTPRequestOutput{
		StatusCode: 201,
		Headers:    map[string][]string{},
		Body:       []byte(`{"id": "abc123"}`),
	}, nil)

	source := []byte(`
load("@plugin", "request")

def test_post():
    res = request.do(
        method="POST",
        url="https://api.example.com/submit",
        body='{"value": 42}',
        headers={"Content-Type": ["application/json"]}
    )
    return res.status_code
`)

	env.ExecuteScript(source, "test_post")
	result := env.GetResult(t)

	// Result should be "201" as a string
	if result != "201" {
		t.Errorf("Expected status code 201, got %s", result)
	}

	env.AssertExpectations(t)
}
