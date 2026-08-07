# Corrected Capability ID Rules

## Format
```
source/namespace/name
```

## Segment Rules

### Source
- Must be one of the predefined Kernel capability sources
- Examples: `builtin`, `external`, `plugin`, `mcp`, `workflow`, `computer_use`, `provider`, `internal`, `legacy`

### Namespace
- Represents stable capability domain
- Derived from B9 domain mapping
- Examples: `tool`, `system`, `browser`, `memory`, `process`, `security`, `device`, `conversation`, `search`, `extension`, `character`, `network`, `voice`, `notification`, `file`, `model`, `agent`, `task`

### Name
- Represents executable behavior
- Must use verb-object pattern
- Must be snake_case ASCII
- Examples: `resolve_interaction_scope`, `update_full_apk`, `write_memory`, `capture_screenshot`

## Constraints
- ASCII only
- Lowercase only
- Max 3 segments separated by `/`
- Allowed chars: `a-z`, `0-9`, `/`, `.`, `_`, `-`
- No Chinese characters
- No spaces
- No parentheses
- No implementation class names

## Kernel Reference
- Builder: `BuildCapabilityID(source CapabilitySource, namespace, name string) string`
- Location: `backend/internal/extension/kernel/capability/id.go`
- Validation: `asciiOnlyPattern = regexp.MustCompile(`[^a-z0-9/._-]`)`
