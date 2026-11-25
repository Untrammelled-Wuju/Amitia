//go:build rsi_routeconformance

package routeconformance

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

type FieldContract struct {
	Name string
	JSON string
	Type string
}

type MethodContract struct {
	Name    string
	Inputs  []string
	Outputs []string
}

func FrozenBindingFields() []FieldContract {
	return []FieldContract{
		{Name: "Key", JSON: "key", Type: "plugincontract.BindingKey"},
		{Name: "ProviderID", JSON: "providerId", Type: "string"},
		{Name: "ProviderInstanceID", JSON: "providerInstanceId", Type: "string"},
	}
}

func FrozenBindingKeyFields() []FieldContract {
	return []FieldContract{
		{Name: "CapabilityID", JSON: "capabilityId", Type: "string"},
		{Name: "SpaceID", JSON: "spaceId", Type: "string"},
		{Name: "DeviceID", JSON: "deviceId", Type: "string"},
		{Name: "Scope", JSON: "scope", Type: "plugincontract.BindingScope"},
	}
}

func FrozenBindingRequestFields() []FieldContract {
	return []FieldContract{
		{Name: "CapabilityID", JSON: "capabilityId", Type: "string"},
		{Name: "SpaceID", JSON: "spaceId", Type: "string"},
		{Name: "DeviceID", JSON: "deviceId", Type: "string"},
	}
}

func FrozenScopeValues() []string {
	return []string{string(plugincontract.BindingScopeDevice), string(plugincontract.BindingScopeSpace), string(plugincontract.BindingScopeGlobal)}
}

func FrozenSnapshotMethods() []MethodContract {
	return []MethodContract{
		{Name: "Bindings", Outputs: []string{"[]plugincontract.Binding"}},
		{Name: "Digest", Outputs: []string{"string"}},
		{Name: "Epoch", Outputs: []string{"string"}},
		{Name: "Len", Outputs: []string{"int"}},
		{Name: "Lookup", Inputs: []string{"plugincontract.BindingKey"}, Outputs: []string{"plugincontract.Binding", "bool"}},
		{Name: "Resolve", Inputs: []string{"plugincontract.BindingRequest"}, Outputs: []string{"plugincontract." + "BindingResolution", "error"}},
		{Name: "Revision", Outputs: []string{"uint64"}},
	}
}

func GoldenBindingJSON() string {
	return `{"key":{"capabilityId":"memory.search","spaceId":"space-a","deviceId":"device-1","scope":"device"},"providerId":"provider.a","providerInstanceId":"provider.a:1"}`
}

func DiffStructFields(kind reflect.Type, want []FieldContract) []string {
	problems := make([]string, 0)
	if kind == nil {
		return append(problems, "structure contract has no reflected type")
	}
	if len(want) != kind.NumField() {
		problems = append(problems, fmt.Sprintf("%s field count=%d want %d", kind.Name(), kind.NumField(), len(want)))
	}
	limit := len(want)
	if kind.NumField() < limit {
		limit = kind.NumField()
	}
	for index := 0; index < limit; index++ {
		field := kind.Field(index)
		expected := want[index]
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.Name != expected.Name || tag != expected.JSON || field.Type.String() != expected.Type {
			problems = append(problems, fmt.Sprintf(
				"%s field[%d]=%s/%s/%s want %s/%s/%s",
				kind.Name(), index, field.Name, tag, field.Type.String(), expected.Name, expected.JSON, expected.Type,
			))
		}
	}
	return problems
}

func DiffMethods(kind reflect.Type, want []MethodContract) []string {
	problems := make([]string, 0)
	if kind == nil {
		return append(problems, "method contract has no reflected type")
	}
	pointer := reflect.PointerTo(kind)
	actual := make([]MethodContract, 0, pointer.NumMethod())
	for index := 0; index < pointer.NumMethod(); index++ {
		method := pointer.Method(index)
		contract := MethodContract{Name: method.Name}
		for in := 1; in < method.Type.NumIn(); in++ {
			contract.Inputs = append(contract.Inputs, method.Type.In(in).String())
		}
		for out := 0; out < method.Type.NumOut(); out++ {
			contract.Outputs = append(contract.Outputs, method.Type.Out(out).String())
		}
		actual = append(actual, contract)
	}
	sort.Slice(actual, func(left, right int) bool { return actual[left].Name < actual[right].Name })
	if len(actual) != len(want) {
		problems = append(problems, fmt.Sprintf("%s method count=%d want %d", kind.Name(), len(actual), len(want)))
	}
	wanted := make(map[string]MethodContract, len(want))
	for _, contract := range want {
		wanted[contract.Name] = contract
	}
	for _, contract := range actual {
		expected, ok := wanted[contract.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s unexpected method %s", kind.Name(), contract.Name))
			continue
		}
		if strings.Join(contract.Inputs, ",") != strings.Join(expected.Inputs, ",") ||
			strings.Join(contract.Outputs, ",") != strings.Join(expected.Outputs, ",") {
			problems = append(problems, fmt.Sprintf(
				"%s method %s(%s)->(%s) want (%s)->(%s)",
				kind.Name(), contract.Name,
				strings.Join(contract.Inputs, ","), strings.Join(contract.Outputs, ","),
				strings.Join(expected.Inputs, ","), strings.Join(expected.Outputs, ","),
			))
		}
		delete(wanted, contract.Name)
	}
	for name := range wanted {
		problems = append(problems, fmt.Sprintf("%s missing method %s", kind.Name(), name))
	}
	sort.Strings(problems)
	return problems
}

func ContractProblems() []string {
	problems := make([]string, 0)
	problems = append(problems, DiffStructFields(reflect.TypeOf(plugincontract.Binding{}), FrozenBindingFields())...)
	problems = append(problems, DiffStructFields(reflect.TypeOf(plugincontract.BindingKey{}), FrozenBindingKeyFields())...)
	problems = append(problems, DiffStructFields(reflect.TypeOf(plugincontract.BindingRequest{}), FrozenBindingRequestFields())...)
	problems = append(problems, DiffMethods(reflect.TypeOf(plugincontract.BindingSnapshot{}), FrozenSnapshotMethods())...)
	if observed := FrozenScopeValues(); strings.Join(observed, ",") != "device,space,global" {
		problems = append(problems, fmt.Sprintf("scope values=%s want device,space,global", strings.Join(observed, ",")))
	}
	probe := plugincontract.Binding{
		Key: plugincontract.BindingKey{
			CapabilityID: "memory.search",
			SpaceID:      "space-a",
			DeviceID:     "device-1",
			Scope:        plugincontract.BindingScopeDevice,
		},
		ProviderID:         "provider.a",
		ProviderInstanceID: "provider.a:1",
	}
	encoded, err := json.Marshal(probe)
	if err != nil {
		problems = append(problems, fmt.Sprintf("binding json probe failed: %v", err))
	} else if string(encoded) != GoldenBindingJSON() {
		problems = append(problems, fmt.Sprintf("binding json=%s want %s", encoded, GoldenBindingJSON()))
	}
	return problems
}

func GoldenStatus() string {
	probe := plugincontract.Binding{
		Key: plugincontract.BindingKey{
			CapabilityID: "memory.search",
			SpaceID:      "space-a",
			DeviceID:     "device-1",
			Scope:        plugincontract.BindingScopeDevice,
		},
		ProviderID:         "provider.a",
		ProviderInstanceID: "provider.a:1",
	}
	encoded, err := json.Marshal(probe)
	if err != nil || string(encoded) != GoldenBindingJSON() {
		return "drift"
	}
	return "ok"
}
