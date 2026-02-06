package plugin

import (
	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/atexit"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/concurrent"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/hashlib"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/json"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/os"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/progress"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/random"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/request"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/script"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/test"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/time"
	"github.com/cadence-workflow/starlark-worker/safeclaw/plugin/uuid"
)

// DefaultPlugins returns the standard set of safeclaw plugins.
func DefaultPlugins() []safeclaw.Plugin {
	return []safeclaw.Plugin{
		atexit.Plugin,
		concurrent.Plugin,
		hashlib.Plugin,
		json.Plugin,
		os.Plugin,
		progress.Plugin,
		random.Plugin,
		request.Plugin,
		script.Plugin,
		test.Plugin,
		time.Plugin,
		uuid.Plugin,
	}
}
