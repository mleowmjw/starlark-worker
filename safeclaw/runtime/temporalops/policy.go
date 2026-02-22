package temporalops

import (
	"strings"
)

// ScriptPolicy defines command restrictions for script execution in non-dev modes.
type ScriptPolicy struct {
	Allowlist []string `json:"allowlist,omitempty"`
}

func DefaultScriptAllowlist() []string {
	return []string{
		"tr",
		"sed",
		"grep",
		"awk",
		"cut",
		"sort",
		"uniq",
		"head",
		"tail",
		"wc",
		"cat",
		"jq",
	}
}

func normalizeAllowlist(allowlist []string) map[string]struct{} {
	res := make(map[string]struct{}, len(allowlist))
	for _, cmd := range allowlist {
		cmd = strings.TrimSpace(strings.ToLower(cmd))
		if cmd == "" {
			continue
		}
		res[cmd] = struct{}{}
	}
	return res
}
