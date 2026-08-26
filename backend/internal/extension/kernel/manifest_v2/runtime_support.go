package manifest_v2

// These sets are the single source of truth for canonical Manifest v2
// module/runtime kinds understood by the extension kernel. Values are
// deliberately matched exactly: schema validation and downstream dispatch use
// these canonical wire values, so accepting case/whitespace aliases here would
// create a preview/runtime split-brain.
var supportedModuleTypes = map[string]struct{}{
	"builtin":    {},
	"javascript": {},
	"data_only":  {},
	"wasm":       {},
	"native":     {},
	"service":    {},
}

var executableModuleTypes = map[string]struct{}{
	"builtin":    {},
	"javascript": {},
	"wasm":       {},
	"native":     {},
	"service":    {},
}

var supportedRuntimeTypes = map[string]struct{}{
	"javascript": {},
	"mcp":        {},
	"workflow":   {},
	"static":     {},
	"wasm":       {},
	"service":    {},
}

func IsSupportedModuleType(value string) bool {
	_, ok := supportedModuleTypes[value]
	return ok
}

func IsSupportedRuntimeType(value string) bool {
	_, ok := supportedRuntimeTypes[value]
	return ok
}

// IsExecutableModuleType reports whether a module kind is expected to carry
// executable code. Keep this in the same canonical table as module/runtime
// support so normalization, validation, preview and dispatch cannot drift.
func IsExecutableModuleType(value string) bool {
	_, ok := executableModuleTypes[value]
	return ok
}
