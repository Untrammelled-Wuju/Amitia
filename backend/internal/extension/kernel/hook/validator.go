package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type PatchValidator struct {
	MaxOperations int
	MaxPathLength int
}

func NewPatchValidator() *PatchValidator {
	return &PatchValidator{
		MaxOperations: 32,
		MaxPathLength: 256,
	}
}

type ValidationError struct {
	Code    HookErrorCode
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (path: %s)", e.Code, e.Message, e.Path)
}

type ValidationContext struct {
	Point       HookPointDefinition
	Contrib     HookContributionDefinition
	CurrentObj  map[string]any
	WrittenPaths map[string]string
}

func (v *PatchValidator) Validate(result HookResult, vctx *ValidationContext) []ValidationError {
	var errs []ValidationError

	if err := result.ValidateSize(vctx.Point.MaxResultBytes); err != nil {
		errs = append(errs, ValidationError{
			Code:    ErrCodeHookResultTooLarge,
			Message: err.Error(),
		})
		return errs
	}

	if err := result.ValidatePatchLimits(v.MaxOperations, v.MaxPathLength); err != nil {
		if he, ok := err.(*HookError); ok {
			errs = append(errs, ValidationError{
				Code:    he.Code,
				Message: he.Message,
			})
		} else {
			errs = append(errs, ValidationError{
				Code:    ErrCodeHookResultInvalid,
				Message: err.Error(),
			})
		}
		return errs
	}

	if !result.Decision.AllowedForPhase(vctx.Contrib.Phase) {
		errs = append(errs, ValidationError{
			Code:    ErrCodeInvalidDecision,
			Message: fmt.Sprintf("decision %s not allowed for phase %s", result.Decision, vctx.Contrib.Phase),
		})
		return errs
	}

	if result.Decision != DecisionReplace && result.Decision != DecisionContinue {
		return errs
	}

	for _, op := range result.Patch {
		normalizedPath := normalizePath(op.Path)
		if !v.isPathWhitelisted(normalizedPath, vctx) {
			errs = append(errs, ValidationError{
				Code:    ErrCodeHookPathNotWhitelisted,
				Path:    normalizedPath,
				Message: "path not in mutation whitelist",
			})
			continue
		}

		if vctx.Point.IsSensitive(normalizedPath) {
			errs = append(errs, ValidationError{
				Code:    ErrCodeHookSensitiveField,
				Path:    normalizedPath,
				Message: "path is a sensitive field",
			})
			continue
		}

		rule, ok := vctx.Point.FindMutationRule(normalizedPath)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    ErrCodeHookPathNotWhitelisted,
				Path:    normalizedPath,
				Message: "no mutation rule for path",
			})
			continue
		}

		opAllowed := false
		for _, allowed := range rule.Operations {
			if allowed == op.Operation {
				opAllowed = true
				break
			}
		}
		if !opAllowed {
			errs = append(errs, ValidationError{
				Code:    ErrCodeHookResultInvalid,
				Path:    normalizedPath,
				Message: fmt.Sprintf("operation %s not allowed for path", op.Operation),
			})
			continue
		}

		claimExists := false
		for _, claim := range vctx.Contrib.MutationClaims {
			if matchPath(claim, normalizedPath) {
				claimExists = true
				break
			}
		}
		if !claimExists && len(vctx.Contrib.MutationClaims) > 0 {
			errs = append(errs, ValidationError{
				Code:    ErrCodeMutationClaimDenied,
				Path:    normalizedPath,
				Message: "path not in contribution mutation claims",
			})
			continue
		}

		if rule.ConflictMode == ConflictExclusive {
			if owner, exists := vctx.WrittenPaths[normalizedPath]; exists && owner != vctx.Contrib.ContributionID {
				errs = append(errs, ValidationError{
					Code:    ErrCodeHookMutationConflict,
					Path:    normalizedPath,
					Message: fmt.Sprintf("exclusive field already written by %s", owner),
				})
				continue
			}
		}

		if len(errs) == 0 {
			vctx.WrittenPaths[normalizedPath] = vctx.Contrib.ContributionID
		}
	}

	return errs
}

func (v *PatchValidator) isPathWhitelisted(path string, vctx *ValidationContext) bool {
	for _, rule := range vctx.Point.AllowedMutations {
		if matchPath(rule.Path, path) {
			return true
		}
	}
	return false
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}

func ApplyPatch(obj map[string]any, patch []MutationOperation) (map[string]any, error) {
	if obj == nil {
		obj = make(map[string]any)
	}
	result, err := deepCopy(obj)
	if err != nil {
		return nil, fmt.Errorf("hook: deep copy: %w", err)
	}
	for _, op := range patch {
		if err := applyOperation(result, op); err != nil {
			return nil, fmt.Errorf("hook: apply %s on %s: %w", op.Operation, op.Path, err)
		}
	}
	return result, nil
}

func applyOperation(obj map[string]any, op MutationOperation) error {
	normalizedPath := normalizePath(op.Path)
	if normalizedPath == "/" {
		return fmt.Errorf("hook: root path not allowed")
	}
	parts := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	current := any(obj)
	for i, part := range parts {
		last := i == len(parts)-1
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("hook: path %s does not point to a map", part)
		}
		if last {
			switch op.Operation {
			case MutationReplace:
				var val any
				if err := json.Unmarshal(op.Value, &val); err != nil {
					return fmt.Errorf("hook: unmarshal value: %w", err)
				}
				m[part] = val
			case MutationAdd:
				var val any
				if err := json.Unmarshal(op.Value, &val); err != nil {
					return fmt.Errorf("hook: unmarshal value: %w", err)
				}
				m[part] = val
			case MutationRemove:
				delete(m, part)
			}
			return nil
		}
		next, exists := m[part]
		if !exists {
			m[part] = make(map[string]any)
			next = m[part]
		}
		current = next
	}
	return nil
}

func deepCopy(obj map[string]any) (map[string]any, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func HashPayload(payload json.RawMessage) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

func HashResult(result HookResult) string {
	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
