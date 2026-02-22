package workflow

import (
	"time"
)

// Backend defines the workflow execution backend interface.
// Implementations can be local (direct execution) or Temporal (workflow-backed).
type Backend interface {
	// InWorkflow returns true if currently executing in a workflow context.
	InWorkflow() bool

	// Now returns the current time. In workflow context, this is deterministic.
	Now() time.Time

	// Sleep suspends execution for the given duration.
	Sleep(d time.Duration) error

	// SideEffect records non-deterministic values for replay.
	SideEffect(f func() any) EncodedValue

	// ExecuteActivity executes an activity and returns a future.
	ExecuteActivity(activity any, args ...any) Future
}

// EncodedValue represents an encoded value that can be retrieved.
type EncodedValue interface {
	Get(valuePtr any) error
}

// Future represents an asynchronous result.
type Future interface {
	Get(valuePtr any) error
	IsReady() bool
}

// GetBackend retrieves the workflow backend from the context.
// Returns nil if no backend is attached.
// Works with both standard context.Context and Temporal's workflow.Context.
func GetBackend(ctx any) Backend {
	// Try to get the value using the Value method (works for both types)
	type valuer interface {
		Value(key any) any
	}
	if v, ok := ctx.(valuer); ok {
		if backend, ok := v.Value("safeclaw.workflow.backend").(Backend); ok {
			return backend
		}
	}
	return nil
}

// Mode represents the execution mode based on environment.
type Mode int

const (
	// ModeDirect uses direct Go SDK calls (dev/local)
	ModeDirect Mode = iota
	// ModeWorkflow uses Temporal workflow APIs (staging/production)
	ModeWorkflow
)

// GetMode determines the execution mode from environment variables.
// Takes a map of environment variables to avoid circular dependency.
func GetMode(environ map[string]string) Mode {
	mode := environ["SAFECLAW_ENV"]
	if mode == "" {
		mode = environ["CHAMELEON_MODE"]
	}
	switch mode {
	case "staging", "prod", "production":
		return ModeWorkflow
	default:
		return ModeDirect
	}
}

// IsTestMode checks if we're in test mode (for synctest compatibility).
func IsTestMode(environ map[string]string) bool {
	return environ["SAFECLAW_TEST"] == "true"
}
