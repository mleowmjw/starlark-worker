package mode

import "strings"

const (
	EnvChameleonMode = "CHAMELEON_MODE"
	EnvSafeclawEnv   = "SAFECLAW_ENV"
)

type Value string

const (
	Dev     Value = "dev"
	Staging Value = "staging"
	Prod    Value = "prod"
	Test    Value = "test"
)

// Resolve normalizes environment mode values used by Safeclaw plugins.
// Priority is CHAMELEON_MODE first, then SAFECLAW_ENV, then default dev.
func Resolve(environ map[string]string) Value {
	raw := strings.TrimSpace(environ[EnvChameleonMode])
	if raw == "" {
		raw = strings.TrimSpace(environ[EnvSafeclawEnv])
	}
	return Normalize(raw)
}

func Normalize(raw string) Value {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default", "dev", "development":
		return Dev
	case "stage", "staging":
		return Staging
	case "prod", "production":
		return Prod
	case "test", "testing":
		return Test
	default:
		return Dev
	}
}
