package request

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// HTTPRequestInput contains the parameters for an HTTP request activity.
type HTTPRequestInput struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string][]string
}

// HTTPRequestOutput contains the serialized HTTP response.
type HTTPRequestOutput struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// HTTPRequestActivity executes an HTTP request as a Temporal activity.
// This is called by workers, not in workflow code.
func HTTPRequestActivity(ctx context.Context, input HTTPRequestInput) (*HTTPRequestOutput, error) {
	// Create HTTP request
	var bodyReader io.Reader
	if len(input.Body) > 0 {
		bodyReader = bytes.NewReader(input.Body)
	}
	
	req, err := http.NewRequestWithContext(ctx, input.Method, input.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header = http.Header(input.Headers)
	
	// Execute the request
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Convert headers to map
	headers := make(map[string][]string)
	for k, v := range res.Header {
		headers[k] = v
	}
	
	return &HTTPRequestOutput{
		StatusCode: res.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}
