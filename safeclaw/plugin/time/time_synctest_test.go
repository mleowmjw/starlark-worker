package time_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
	"go.starlark.net/starlark"
)

// TestTimeSleepSynctest tests time.sleep with synctest for deterministic time control.
func TestTimeSleepSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

		source := []byte(`
load("@plugin", "time")

def test_sleep():
    start = time.time()
    time.sleep(5)
    end = time.time()
    return end - start
`)

		resultCh := make(chan float64)
		errCh := make(chan error)

		go func() {
			result, err := runner.RunSource(context.Background(), source, "test_sleep")
			if err != nil {
				errCh <- err
				return
			}
			if f, ok := result.(starlark.Float); ok {
				resultCh <- float64(f)
			} else {
				errCh <- nil
			}
		}()

		// Wait for goroutine to block on sleep
		synctest.Wait()

		// Advance fake time by 5 seconds
		time.Sleep(5 * time.Second)

		// Get result
		select {
		case elapsed := <-resultCh:
			// Should be approximately 5 seconds
			if elapsed < 4.9 || elapsed > 5.1 {
				t.Errorf("Expected ~5 seconds, got %f", elapsed)
			}
		case err := <-errCh:
			t.Fatalf("Execution failed: %v", err)
		}
	})
}

// TestTimeNowSynctest tests that time.time() advances with synctest.
func TestTimeNowSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

		source := []byte(`
load("@plugin", "time")

def get_time():
    return time.time()
`)

		// Get initial time
		result1, err := runner.RunSource(context.Background(), source, "get_time")
		if err != nil {
			t.Fatalf("First execution failed: %v", err)
		}
		time1 := float64(result1.(starlark.Float))

		// Advance time by 10 seconds
		time.Sleep(10 * time.Second)

		// Get time again
		result2, err := runner.RunSource(context.Background(), source, "get_time")
		if err != nil {
			t.Fatalf("Second execution failed: %v", err)
		}
		time2 := float64(result2.(starlark.Float))

		// Time should have advanced by approximately 10 seconds
		elapsed := time2 - time1
		if elapsed < 9.9 || elapsed > 10.1 {
			t.Errorf("Expected ~10 seconds elapsed, got %f", elapsed)
		}
	})
}

// TestConcurrentSleeps tests multiple concurrent sleep operations with synctest.
func TestConcurrentSleeps(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

		source := []byte(`
load("@plugin", "time")

def sleep_n(n):
    time.sleep(n)
    return n
`)

		results := make([]chan float64, 3)
		for i := range results {
			results[i] = make(chan float64)
		}

		// Start 3 concurrent sleeps
		for i, duration := range []int{2, 4, 6} {
			i, duration := i, duration
			go func() {
				result, _ := runner.RunSource(context.Background(), source, "sleep_n", duration)
				if f, ok := result.(starlark.Int); ok {
					results[i] <- float64(f.Float())
				}
			}()
		}

		// Wait for all goroutines to block
		synctest.Wait()

		// Advance time to complete all sleeps
		time.Sleep(6 * time.Second)

		// All should complete
		for i := 0; i < 3; i++ {
			select {
			case <-results[i]:
				// Success
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Goroutine %d did not complete", i)
			}
		}
	})
}
