package chameleon

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

// Plugin is a flexible Chameleon plugin for mocking any service interface.
//
// The Chameleon plugin allows defining mock behaviors at runtime, enabling
// rapid prototyping of new service integrations before implementing real backends.
//
// Environment variables:
// - CHAMELEON_SCENARIO: default scenario name (default: "default")
// - CHAMELEON_MODE: "dev" | "staging" | "prod" (default: "dev")
//
// Starlark API (module = `chameleon`):
// - chameleon.info() -> dict
// - chameleon.set_scenario(name) -> bool
// - chameleon.get_scenario() -> str
// - chameleon.scenarios() -> [str]
// - chameleon.register_service(name, scenarios_json) -> bool
// - chameleon.call(service, method, args_json) -> dict
// - chameleon.get_mock_data(service, key) -> any
var Plugin safeclaw.Plugin = &plugin{}

type plugin struct{}

func (p *plugin) ID() string { return "chameleon" }

func (p *plugin) Module(ctx context.Context, info safeclaw.RunInfo) starlark.Value {
	scenario := strings.TrimSpace(info.Environ["CHAMELEON_SCENARIO"])
	if scenario == "" {
		scenario = "default"
	}
	mode := strings.TrimSpace(info.Environ["CHAMELEON_MODE"])
	if mode == "" {
		mode = strings.TrimSpace(info.Environ["SAFECLAW_ENV"])
	}
	if mode == "" {
		mode = "dev"
	}
	return &Module{
		scenario:  scenario,
		mode:      mode,
		startUnix: info.StartTime.Unix(),
		services:  make(map[string]*ServiceMock),
	}
}

type Module struct {
	scenario  string
	mode      string
	startUnix int64
	mu        sync.RWMutex
	services  map[string]*ServiceMock
}

type ServiceMock struct {
	Name      string
	Scenarios map[string]*ScenarioData
}

type ScenarioData struct {
	Description string
	Methods     map[string]json.RawMessage
	Data        map[string]json.RawMessage
}

var _ starlark.HasAttrs = (*Module)(nil)

func (m *Module) String() string        { return "chameleon" }
func (m *Module) Type() string          { return "chameleon" }
func (m *Module) Freeze()               {}
func (m *Module) Truth() starlark.Bool  { return true }
func (m *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: chameleon") }
func (m *Module) Attr(n string) (starlark.Value, error) {
	return star.Attr(m, n, builtins, properties)
}
func (m *Module) AttrNames() []string { return star.AttrNames(builtins, properties) }

var properties = map[string]star.PropertyFactory{}

var builtins = map[string]*starlark.Builtin{
	"info":             starlark.NewBuiltin("info", _info),
	"set_scenario":     starlark.NewBuiltin("set_scenario", _setScenario),
	"get_scenario":     starlark.NewBuiltin("get_scenario", _getScenario),
	"scenarios":        starlark.NewBuiltin("scenarios", _scenarios),
	"register_service": starlark.NewBuiltin("register_service", _registerService),
	"call":             starlark.NewBuiltin("call", _call),
	"get_mock_data":    starlark.NewBuiltin("get_mock_data", _getMockData),
}

func _info(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("info", args, kwargs); err != nil {
		return nil, err
	}
	m := fn.Receiver().(*Module)
	d := starlark.NewDict(4)
	_ = d.SetKey(starlark.String("mode"), starlark.String(m.mode))
	_ = d.SetKey(starlark.String("scenario"), starlark.String(m.scenario))
	_ = d.SetKey(starlark.String("start_unix"), starlark.MakeInt64(m.startUnix))

	m.mu.RLock()
	serviceNames := make([]string, 0, len(m.services))
	for name := range m.services {
		serviceNames = append(serviceNames, name)
	}
	m.mu.RUnlock()
	sort.Strings(serviceNames)
	services := make([]starlark.Value, 0, len(serviceNames))
	for _, name := range serviceNames {
		services = append(services, starlark.String(name))
	}
	_ = d.SetKey(starlark.String("services"), starlark.NewList(services))

	return d, nil
}

func _setScenario(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	if err := starlark.UnpackArgs("set_scenario", args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	m := fn.Receiver().(*Module)
	m.scenario = name.GoString()
	return starlark.Bool(true), nil
}

func _getScenario(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("get_scenario", args, kwargs); err != nil {
		return nil, err
	}
	m := fn.Receiver().(*Module)
	return starlark.String(m.scenario), nil
}

func _scenarios(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var service starlark.String
	if err := starlark.UnpackArgs("scenarios", args, kwargs, "service?", &service); err != nil {
		return nil, err
	}

	m := fn.Receiver().(*Module)
	m.mu.RLock()
	defer m.mu.RUnlock()

	if service.GoString() == "" {
		// Return built-in scenarios
		return starlark.NewList([]starlark.Value{
			starlark.String("default"),
			starlark.String("good_deals"),
			starlark.String("limited_availability"),
			starlark.String("price_drop"),
			starlark.String("sold_out"),
			starlark.String("error"),
			starlark.String("slow_response"),
			starlark.String("flash_sale"),
		}), nil
	}

	svc, ok := m.services[service.GoString()]
	if !ok {
		return starlark.NewList(nil), nil
	}

	scenarioNames := make([]string, 0, len(svc.Scenarios))
	for name := range svc.Scenarios {
		scenarioNames = append(scenarioNames, name)
	}
	sort.Strings(scenarioNames)
	scenarios := make([]starlark.Value, 0, len(scenarioNames))
	for _, name := range scenarioNames {
		scenarios = append(scenarios, starlark.String(name))
	}
	return starlark.NewList(scenarios), nil
}

func _registerService(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	var scenariosJSON starlark.String
	if err := starlark.UnpackArgs("register_service", args, kwargs, "name", &name, "scenarios_json", &scenariosJSON); err != nil {
		return nil, err
	}

	m := fn.Receiver().(*Module)

	var scenarios map[string]*ScenarioData
	if err := json.Unmarshal([]byte(scenariosJSON.GoString()), &scenarios); err != nil {
		return nil, fmt.Errorf("failed to parse scenarios JSON: %w", err)
	}

	m.mu.Lock()
	m.services[name.GoString()] = &ServiceMock{
		Name:      name.GoString(),
		Scenarios: scenarios,
	}
	m.mu.Unlock()

	return starlark.Bool(true), nil
}

func _call(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var service, method, argsJSON starlark.String
	if err := starlark.UnpackArgs("call", args, kwargs, "service", &service, "method", &method, "args_json?", &argsJSON); err != nil {
		return nil, err
	}

	m := fn.Receiver().(*Module)
	m.mu.RLock()
	svc, ok := m.services[service.GoString()]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("service %q not registered", service.GoString())
	}

	scenario, ok := svc.Scenarios[m.scenario]
	if !ok {
		scenario, ok = svc.Scenarios["default"]
		if !ok {
			return nil, fmt.Errorf("scenario %q not found for service %q", m.scenario, service.GoString())
		}
	}

	methodData, ok := scenario.Methods[method.GoString()]
	if !ok {
		return nil, fmt.Errorf("method %q not found in scenario %q for service %q", method.GoString(), m.scenario, service.GoString())
	}

	// Parse JSON result and convert to Starlark
	var result any
	if err := json.Unmarshal(methodData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse method result: %w", err)
	}

	return star.ToStarlark(result)
}

func _getMockData(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var service, key starlark.String
	if err := starlark.UnpackArgs("get_mock_data", args, kwargs, "service", &service, "key", &key); err != nil {
		return nil, err
	}

	m := fn.Receiver().(*Module)
	m.mu.RLock()
	svc, ok := m.services[service.GoString()]
	m.mu.RUnlock()

	if !ok {
		return starlark.None, nil
	}

	scenario, ok := svc.Scenarios[m.scenario]
	if !ok {
		scenario, ok = svc.Scenarios["default"]
		if !ok {
			return starlark.None, nil
		}
	}

	data, ok := scenario.Data[key.GoString()]
	if !ok {
		return starlark.None, nil
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return starlark.None, nil
	}

	return star.ToStarlark(result)
}
