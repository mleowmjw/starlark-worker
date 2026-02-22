package workflow_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw/workflow"
)

// TestLocalBackendSynctest tests the local backend with synctest for deterministic time.
func TestLocalBackendSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		backend := workflow.NewLocalBackend(ctx)

		// Test Now()
		start := backend.Now()

		// Advance time using synctest
		time.Sleep(5 * time.Second)

		after := backend.Now()

		// In synctest, time advances deterministically
		if after.Sub(start) < 5*time.Second {
			t.Errorf("Expected time to advance by at least 5 seconds, got %v", after.Sub(start))
		}
	})
}

func TestLocalBackendSleep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		backend := workflow.NewLocalBackend(ctx)

		done := make(chan bool)

		go func() {
			err := backend.Sleep(3 * time.Second)
			if err != nil {
				t.Errorf("Sleep failed: %v", err)
			}
			done <- true
		}()

		// Wait for goroutine to block on sleep
		synctest.Wait()

		// Advance time
		time.Sleep(3 * time.Second)

		// Wait for completion
		<-done
	})
}

func TestLocalBackendSideEffect(t *testing.T) {
	backend := workflow.NewLocalBackend(context.Background())

	// Test that SideEffect executes immediately
	called := false
	result := backend.SideEffect(func() any {
		called = true
		return 42
	})

	if !called {
		t.Error("Expected SideEffect to execute immediately in local mode")
	}

	var value int
	err := result.Get(&value)
	if err != nil {
		t.Errorf("Failed to get SideEffect value: %v", err)
	}

	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}
}

func TestLocalBackendInWorkflow(t *testing.T) {
	backend := workflow.NewLocalBackend(context.Background())

	if backend.InWorkflow() {
		t.Error("Local backend should return false for InWorkflow()")
	}
}

func TestGetModeFromEnviron(t *testing.T) {
	tests := []struct {
		name     string
		environ  map[string]string
		expected workflow.Mode
	}{
		{
			name:     "dev mode default",
			environ:  map[string]string{},
			expected: workflow.ModeDirect,
		},
		{
			name:     "staging mode",
			environ:  map[string]string{"SAFECLAW_ENV": "staging"},
			expected: workflow.ModeWorkflow,
		},
		{
			name:     "production mode",
			environ:  map[string]string{"SAFECLAW_ENV": "production"},
			expected: workflow.ModeWorkflow,
		},
		{
			name:     "prod shorthand",
			environ:  map[string]string{"SAFECLAW_ENV": "prod"},
			expected: workflow.ModeWorkflow,
		},
		{
			name:     "chameleon mode staging",
			environ:  map[string]string{"CHAMELEON_MODE": "staging"},
			expected: workflow.ModeWorkflow,
		},
		{
			name:     "dev mode explicit",
			environ:  map[string]string{"SAFECLAW_ENV": "dev"},
			expected: workflow.ModeDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := workflow.GetMode(tt.environ)
			if mode != tt.expected {
				t.Errorf("Expected mode %v, got %v", tt.expected, mode)
			}
		})
	}
}
