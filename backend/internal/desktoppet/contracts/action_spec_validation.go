// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var actionKeyRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

type ValidationError struct {
	ActionKey string
	Field     string
	Message   string
}

func (e ValidationError) Error() string {
	if e.ActionKey != "" {
		return fmt.Sprintf("action %s: %s: %s", e.ActionKey, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ValidateActionSpec(spec ActionSpec) []ValidationError {
	var errs []ValidationError
	key := spec.Identity.Key

	if !actionKeyRegex.MatchString(key) || len(key) > 64 {
		errs = append(errs, ValidationError{key, "key", "invalid format or length"})
	}

	if strings.TrimSpace(spec.Identity.Name) == "" {
		errs = append(errs, ValidationError{key, "name", "must be non-empty"})
	}

	if _, ok := CategoryByKey(spec.Identity.CategoryKey); !ok {
		errs = append(errs, ValidationError{key, "categoryKey", "category does not exist"})
	}

	if spec.Identity.CategorySortOrder < 0 || spec.Identity.ActionSortOrder < 0 {
		errs = append(errs, ValidationError{key, "sortOrder", "must be non-negative"})
	}

	fc := spec.Generation.FrameCount
	if fc < 1 || fc > 120 {
		errs = append(errs, ValidationError{key, "frameCount", "must be 1-120"})
	}

	if len(spec.Generation.FramePhases) != fc {
		errs = append(errs, ValidationError{key, "framePhases", "count must equal frameCount"})
	}

	for i, phase := range spec.Generation.FramePhases {
		if phase.Index != i {
			errs = append(errs, ValidationError{key, fmt.Sprintf("framePhases[%d].index", i), fmt.Sprintf("expected %d got %d", i, phase.Index)})
		}
	}

	fps := spec.Playback.DefaultFPS
	if fps < 1 || fps > 60 {
		errs = append(errs, ValidationError{key, "defaultFps", "must be 1-60"})
	}

	if !IsValidPlaybackMode(string(spec.Playback.Mode)) {
		errs = append(errs, ValidationError{key, "playbackMode", "invalid mode"})
	}

	if spec.Identity.SupportsDefaultIdle {
		if spec.Playback.Mode != PlaybackLoop && spec.Playback.Mode != PlaybackPingPong {
			errs = append(errs, ValidationError{key, "supportsDefaultIdle", "must be loop or ping_pong"})
		}
	}

	switch spec.Playback.Mode {
	case PlaybackOnce:
		if spec.Playback.ReturnPolicy == ReturnNone {
			errs = append(errs, ValidationError{key, "returnPolicy", "once must have explicit return policy"})
		}
	case PlaybackLoop:
		if spec.Playback.ReturnPolicy == "" {
			errs = append(errs, ValidationError{key, "returnPolicy", "loop must have explicit return policy"})
		}
	}

	if spec.Playback.ReturnPolicy == ReturnSpecific && strings.TrimSpace(spec.Playback.ReturnActionKey) == "" {
		errs = append(errs, ValidationError{key, "returnActionKey", "specific policy requires target"})
	}

	minMS := spec.Playback.MinimumPlayMS
	maxMS := spec.Playback.MaximumPlayMS
	if minMS < 0 {
		errs = append(errs, ValidationError{key, "minimumPlayMs", "must be non-negative"})
	}
	if maxMS != 0 && maxMS < minMS {
		errs = append(errs, ValidationError{key, "maximumPlayMs", "must be 0 or >= minimumPlayMs"})
	}
	if maxMS > 0 && spec.Playback.InterruptAfterMS > maxMS {
		errs = append(errs, ValidationError{key, "interruptAfterMs", "must be <= maximumPlayMs when max>0"})
	}

	pri := spec.Playback.Priority
	if pri < 0 || pri > 100 {
		errs = append(errs, ValidationError{key, "priority", "must be 0-100"})
	}

	if spec.Playback.CooldownMS < 0 {
		errs = append(errs, ValidationError{key, "cooldownMs", "must be non-negative"})
	}
	if spec.Playback.DedupWindowMS < 0 {
		errs = append(errs, ValidationError{key, "dedupWindowMs", "must be non-negative"})
	}

	if spec.Identity.Source == ActionSourceBuiltin {
		if strings.TrimSpace(spec.Playback.MutexGroup) == "" {
			errs = append(errs, ValidationError{key, "mutexGroup", "builtin must have mutex group"})
		}
	}

	if !IsValidQueuePolicy(string(spec.Playback.QueuePolicy)) {
		errs = append(errs, ValidationError{key, "queuePolicy", "invalid queue policy"})
	}

	if !IsValidAnchorProfile(string(spec.Processing.AnchorProfile)) {
		errs = append(errs, ValidationError{key, "anchorProfile", "invalid anchor profile"})
	}

	seenTags := make(map[string]bool)
	sortedTags := make([]string, len(spec.Identity.Tags))
	copy(sortedTags, spec.Identity.Tags)
	sort.Strings(sortedTags)
	for _, tag := range spec.Identity.Tags {
		if len(tag) > 64 {
			errs = append(errs, ValidationError{key, "tags", "tag length exceeds 64"})
		}
		if seenTags[tag] {
			errs = append(errs, ValidationError{key, "tags", "duplicate tag: " + tag})
		}
		seenTags[tag] = true
	}

	if spec.SchemaVersion != ActionSpecSchemaVersion {
		errs = append(errs, ValidationError{key, "schemaVersion", fmt.Sprintf("expected %d got %d", ActionSpecSchemaVersion, spec.SchemaVersion)})
	}

	return errs
}

func ValidateCatalog(specs []ActionSpec) []ValidationError {
	var errs []ValidationError
	seenKeys := make(map[string]bool)
	keyToSpec := make(map[string]ActionSpec)

	for _, spec := range specs {
		ve := ValidateActionSpec(spec)
		errs = append(errs, ve...)

		if seenKeys[spec.Identity.Key] {
			errs = append(errs, ValidationError{spec.Identity.Key, "key", "duplicate action key"})
		}
		seenKeys[spec.Identity.Key] = true
		keyToSpec[spec.Identity.Key] = spec
	}

	hasDefaultIdle := false
	for _, spec := range specs {
		if spec.Identity.SupportsDefaultIdle && spec.Identity.Enabled {
			hasDefaultIdle = true
			break
		}
	}
	if !hasDefaultIdle {
		errs = append(errs, ValidationError{"", "catalog", "must have at least one enabled default idle"})
	}

	for _, spec := range specs {
		if spec.Playback.ReturnPolicy == ReturnSpecific {
			target := spec.Playback.ReturnActionKey
			if !seenKeys[target] {
				errs = append(errs, ValidationError{spec.Identity.Key, "returnActionKey", "target does not exist: " + target})
			}
		}
	}

	if cycle := detectReturnCycle(specs); cycle != "" {
		errs = append(errs, ValidationError{"", "returnGraph", "cycle detected: " + cycle})
	}

	return errs
}

func detectReturnCycle(specs []ActionSpec) string {
	adj := make(map[string]string)
	for _, spec := range specs {
		if spec.Playback.ReturnPolicy == ReturnSpecific {
			adj[spec.Identity.Key] = spec.Playback.ReturnActionKey
		}
	}

	for start := range adj {
		visited := make(map[string]bool)
		current := start
		path := []string{current}
		for {
			next, ok := adj[current]
			if !ok {
				break
			}
			if visited[next] {
				return strings.Join(append(path, next), " -> ")
			}
			visited[current] = true
			current = next
			path = append(path, current)
			if len(path) > len(specs)+1 {
				break
			}
		}
	}
	return ""
}

func NormalizeSpec(spec ActionSpec) ActionSpec {
	if spec.Identity.Tags == nil {
		spec.Identity.Tags = []string{}
	}
	if spec.Generation.FramePhases == nil {
		spec.Generation.FramePhases = []FramePhase{}
	}
	tags := make([]string, len(spec.Identity.Tags))
	copy(tags, spec.Identity.Tags)
	sort.Strings(tags)
	spec.Identity.Tags = tags

	if spec.SchemaVersion == 0 {
		spec.SchemaVersion = ActionSpecSchemaVersion
	}
	return spec
}
