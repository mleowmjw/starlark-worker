package request

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

type plugin struct{}

var Plugin safeclaw.Plugin = &plugin{}

func (p *plugin) ID() string {
	return "request"
}

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	return &Module{
		client: http.DefaultClient,
		ctx:    ctx,
	}
}

type Module struct {
	client *http.Client
	ctx    context.Context
}

var _ starlark.HasAttrs = &Module{}

func (f *Module) String() string                        { return "request" }
func (f *Module) Type() string                          { return "request" }
func (f *Module) Freeze()                               {}
func (f *Module) Truth() starlark.Bool                  { return true }
func (f *Module) Hash() (uint32, error)                 { return 0, fmt.Errorf("unhashable: request") }
func (f *Module) Attr(n string) (starlark.Value, error) { return star.Attr(f, n, builtins, properties) }
func (f *Module) AttrNames() []string                   { return star.AttrNames(builtins, properties) }

var builtins = map[string]*starlark.Builtin{
	"do": starlark.NewBuiltin("do", _do),
}

var properties = map[string]star.PropertyFactory{}

func _do(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var method starlark.String
	var url starlark.String
	var body starlark.Value = starlark.None
	var headers starlark.Value = starlark.None

	if err := starlark.UnpackArgs("do", args, kwargs, "method", &method, "url", &url, "body?", &body, "headers?", &headers); err != nil {
		logger.Error("request.do: unpack args failed", "error", err)
		return nil, err
	}

	// Get module from builtin receiver
	module := b.Receiver().(*Module)

	// Convert body to bytes
	var bodyBytes []byte
	if body != starlark.None {
		switch v := body.(type) {
		case starlark.String:
			bodyBytes = []byte(v)
		case starlark.Bytes:
			bodyBytes = []byte(v)
		default:
			// Try to encode as JSON
			encoded, err := star.Encode(v)
			if err != nil {
				logger.Error("request.do: failed to encode body", "error", err)
				return nil, fmt.Errorf("failed to encode body: %w", err)
			}
			bodyBytes = []byte(encoded)
		}
	}

	// Convert headers to map
	headerMap := make(map[string][]string)
	if headers != starlark.None {
		if dict, ok := headers.(*starlark.Dict); ok {
			for _, item := range dict.Items() {
				key := item[0].(starlark.String).GoString()
				value := item[1]
				
				// Handle both string and list of strings
				switch v := value.(type) {
				case starlark.String:
					headerMap[key] = []string{v.GoString()}
				case *starlark.List:
					vals := make([]string, v.Len())
					for i := 0; i < v.Len(); i++ {
						vals[i] = v.Index(i).(starlark.String).GoString()
					}
					headerMap[key] = vals
				}
			}
		}
	}

	// Create HTTP request
	var br io.Reader
	if len(bodyBytes) > 0 {
		br = bytes.NewBuffer(bodyBytes)
	}
	
	req, err := http.NewRequestWithContext(module.ctx, method.GoString(), url.GoString(), br)
	if err != nil {
		logger.Error("request.do: failed to create request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header = http.Header(headerMap)

	// Execute the request
	res, err := module.client.Do(req)
	if err != nil {
		logger.Error("request.do: request failed", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Serialize the response to bytes (like the original does)
	var buf bytes.Buffer
	if err := res.Write(&buf); err != nil {
		logger.Error("request.do: failed to serialize response", "error", err)
		return nil, fmt.Errorf("failed to serialize response: %w", err)
	}

	// Parse it back to create a Response object
	parsedRes, err := http.ReadResponse(bufio.NewReader(&buf), nil)
	if err != nil {
		logger.Error("request.do: failed to parse response", "error", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &Response{Response: parsedRes}, nil
}

// Response wraps http.Response for Starlark
type Response struct {
	Response *http.Response
}

var _ starlark.Value = &Response{}
var _ starlark.HasAttrs = &Response{}

func (r *Response) String() string        { return fmt.Sprintf("<Response %d>", r.Response.StatusCode) }
func (r *Response) Type() string          { return "Response" }
func (r *Response) Freeze()                {}
func (r *Response) Truth() starlark.Bool  { return true }
func (r *Response) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Response") }

func (r *Response) Attr(name string) (starlark.Value, error) {
	switch name {
	case "status_code":
		return starlark.MakeInt(r.Response.StatusCode), nil
	case "headers":
		headers := starlark.NewDict(len(r.Response.Header))
		for k, v := range r.Response.Header {
			list := starlark.NewList(make([]starlark.Value, len(v)))
			for i, val := range v {
				list.SetIndex(i, starlark.String(val))
			}
			headers.SetKey(starlark.String(k), list)
		}
		return headers, nil
	case "text":
		body, err := io.ReadAll(r.Response.Body)
		if err != nil {
			return nil, err
		}
		return starlark.String(body), nil
	case "json":
		body, err := io.ReadAll(r.Response.Body)
		if err != nil {
			return nil, err
		}
		var result starlark.Value
		if err := star.Decode(body, &result); err != nil {
			return nil, err
		}
		return result, nil
	case "content":
		body, err := io.ReadAll(r.Response.Body)
		if err != nil {
			return nil, err
		}
		return starlark.Bytes(body), nil
	default:
		return nil, nil
	}
}

func (r *Response) AttrNames() []string {
	return []string{"status_code", "headers", "text", "json", "content"}
}
