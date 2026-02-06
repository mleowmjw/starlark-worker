package safeclaw_test

import (
	"context"
	"testing"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin"
)

func TestBasicExecution(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
def add(a, b):
    return a + b
`)

	result, err := runner.RunSource(context.Background(), source, "add", 2, 3)
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "5" {
		t.Errorf("Expected 5, got %s", result.String())
	}
}

func TestJSONPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "json")

def test_json():
    obj = {"name": "test", "value": 42}
    encoded = json.dumps(obj)
    decoded = json.loads(encoded)
    return decoded["value"]
`)

	result, err := runner.RunSource(context.Background(), source, "test_json")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "42" {
		t.Errorf("Expected 42, got %s", result.String())
	}
}

func TestTimePlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "time")

def test_time():
    # Get current time
    current = time.time()
    # Should be a positive number
    return current > 0
`)

	result, err := runner.RunSource(context.Background(), source, "test_time")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "True" {
		t.Errorf("Expected True, got %s", result.String())
	}
}

func TestRandomPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "random")

def test_random():
    random.seed(12345)
    val1 = random.randint(min=1, max=100)
    val2 = random.random()
    return {"int": val1, "float": val2}
`)

	result, err := runner.RunSource(context.Background(), source, "test_random")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Just verify it returns something
	if result.String() == "" {
		t.Error("Expected non-empty result")
	}
}

func TestUUIDPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "uuid")

def test_uuid():
    u = uuid.uuid4()
    return {
        "hex": u.hex,
        "urn": u.urn
    }
`)

	result, err := runner.RunSource(context.Background(), source, "test_uuid")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Verify UUID was generated
	if result.String() == "" {
		t.Error("Expected non-empty UUID result")
	}
}

func TestHashlibPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "hashlib")

def test_hash():
    hash_val = hashlib.blake2b_hex(data="test", digest_size=16)
    return len(hash_val) == 32  # 16 bytes = 32 hex chars
`)

	result, err := runner.RunSource(context.Background(), source, "test_hash")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "True" {
		t.Errorf("Expected True, got %s", result.String())
	}
}

func TestTestPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", t = "test")

def test_assertions():
    t.true(True)
    t.false(False)
    t.equal(expected=5, actual=5)
    t.not_equal(expected=5, actual=10)
    return "all tests passed"
`)

	result, err := runner.RunSource(context.Background(), source, "test_assertions")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "\"all tests passed\"" {
		t.Errorf("Expected 'all tests passed', got %s", result.String())
	}
}

func TestOSPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "os")

def test_environ():
    # Environment should be accessible
    return type(os.environ) == "dict"
`)

	result, err := runner.RunSource(context.Background(), source, "test_environ")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "True" {
		t.Errorf("Expected True, got %s", result.String())
	}
}

func TestProgressPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "progress")

def test_progress():
    progress.report(progress="Starting task")
    progress.report(progress="Task complete")
    return progress.task_state_succeeded
`)

	result, err := runner.RunSource(context.Background(), source, "test_progress")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "\"SUCCEEDED\"" {
		t.Errorf("Expected SUCCEEDED, got %s", result.String())
	}
}

func TestConcurrentPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "concurrent")

def worker(n):
    return n * 2

def test_concurrent():
    futures = []
    for i in range(5):
        f = concurrent.run(worker, i)
        futures.append(f)
    
    results = [f.result() for f in futures]
    total = 0
    for r in results:
        total += r
    return total
`)

	result, err := runner.RunSource(context.Background(), source, "test_concurrent")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// 0*2 + 1*2 + 2*2 + 3*2 + 4*2 = 0 + 2 + 4 + 6 + 8 = 20
	if result.String() != "20" {
		t.Errorf("Expected 20, got %s", result.String())
	}
}

func TestScriptPlugin(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
load("@plugin", "script")

def test_script():
    result = script.echo("hello").string()
    # script.Echo adds a newline automatically
    return "hello" in result
`)

	result, err := runner.RunSource(context.Background(), source, "test_script")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if result.String() != "True" {
		t.Errorf("Expected True, got %s", result.String())
	}
}

func TestDataclass(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
def test_dataclass():
    p = Dataclass(name="Alice", age=30)
    return p.name + "_" + str(p.age)
`)

	result, err := runner.RunSource(context.Background(), source, "test_dataclass")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Verify the dataclass worked
	// result.String() returns the Starlark representation, which includes quotes for strings
	if result.String() != `"Alice_30"` {
		t.Errorf("Expected '\"Alice_30\"', got %s", result.String())
	}
}

func TestCallableObject(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
def test_callable():
    def greet(name):
        return "Hello, " + name
    
    obj = CallableObject(greet)
    obj.custom_attr = "test"
    
    return {
        "result": obj("World"),
        "attr": obj.custom_attr
    }
`)

	result, err := runner.RunSource(context.Background(), source, "test_callable")
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Verify callable object worked
	if result.String() == "" {
		t.Error("Expected non-empty callable object result")
	}
}

func TestErrorHandling(t *testing.T) {
	runner := safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	source := []byte(`
def test_error():
    return 1 / 0  # Division by zero
`)

	_, err := runner.RunSource(context.Background(), source, "test_error")
	if err == nil {
		t.Error("Expected error for division by zero")
	}
}

func TestTarFS(t *testing.T) {
	_ = safeclaw.NewRunner(plugin.DefaultPlugins(), nil)

	// This would require creating a tar file, which we'll skip for brevity
	// but the functionality is tested in example 07_files
	t.Skip("TarFS tested in examples/07_files")
}
