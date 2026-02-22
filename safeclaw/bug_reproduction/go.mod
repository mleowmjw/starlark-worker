module bug_reproduction

go 1.25

replace github.com/cadence-workflow/starlark-worker/safeclaw => ../

require github.com/cadence-workflow/starlark-worker/safeclaw v0.0.0-00010101000000-000000000000

require (
	github.com/bitfield/script v0.24.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/itchyny/gojq v0.12.18 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	go.starlark.net v0.0.0-20260102030733-3fee463870c9 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	mvdan.cc/sh/v3 v3.12.0 // indirect
)
