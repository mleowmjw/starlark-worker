module bug_reproduction

go 1.25

replace github.com/cadence-workflow/starlark-worker/safeclaw => ../

require github.com/cadence-workflow/starlark-worker/safeclaw v0.0.0-00010101000000-000000000000

require (
	github.com/bitfield/script v0.24.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/itchyny/gojq v0.12.13 // indirect
	github.com/itchyny/timefmt-go v0.1.5 // indirect
	go.starlark.net v0.0.0-20241125201518-c05ff208a98f // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	mvdan.cc/sh/v3 v3.7.0 // indirect
)
