package uitree

import "context"

type SourceStatus struct {
	Type      SourceType `json:"type"`
	Available bool       `json:"available"`
	Reason    string     `json:"reason,omitempty"`
}

type RawSnapshot struct {
	Source         SourceType       `json:"source"`
	Generation     int64            `json:"generation"`
	CapturedAt     int64            `json:"capturedAt"`
	RawWindows     []map[string]any `json:"windows,omitempty"`
	RawNodes       []map[string]any `json:"nodes,omitempty"`
	Truncated      bool             `json:"truncated"`
	MultiWindow    bool             `json:"multiWindow"`
	StableRef      bool             `json:"stableRef"`
	AccessibilityOK bool            `json:"accessibilityOk,omitempty"`
}

type Source interface {
	Type() SourceType
	Status(ctx context.Context) SourceStatus
	Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error)
}

type SourceSet struct {
	Accessibility Source
	Root          Source
	ADB           Source
}

func (s SourceSet) AvailableSources() []string {
	var sources []string
	if s.Accessibility != nil {
		status := s.Accessibility.Status(context.Background())
		if status.Available {
			sources = append(sources, string(SourceTypeAccessibility))
		}
	}
	if s.Root != nil {
		status := s.Root.Status(context.Background())
		if status.Available {
			sources = append(sources, string(SourceTypeRoot))
		}
	}
	if s.ADB != nil {
		status := s.ADB.Status(context.Background())
		if status.Available {
			sources = append(sources, string(SourceTypeADB))
		}
	}
	return sources
}

func (s SourceSet) SelectSource(req SnapshotRequest, allowRootFallback bool) (Source, SourceType, error) {
	source := req.Source

	if source == "" || source == SourceAuto {
		if s.Accessibility != nil {
			status := s.Accessibility.Status(context.Background())
			if status.Available {
				return s.Accessibility, SourceTypeAccessibility, nil
			}
		}

		if s.ADB != nil {
			status := s.ADB.Status(context.Background())
			if status.Available {
				return s.ADB, SourceTypeADB, nil
			}
		}

		if allowRootFallback && s.Root != nil {
			status := s.Root.Status(context.Background())
			if status.Available {
				return s.Root, SourceTypeRoot, nil
			}
		}

		return nil, "", &Error{Code: UI_TREE_UNAVAILABLE, Message: "no UI Tree source available"}
	}

	switch source {
	case SourceAccessibility:
		if s.Accessibility == nil {
			return nil, "", &Error{Code: UI_TREE_ACCESSIBILITY_NOT_CONNECTED, Message: "accessibility source not configured"}
		}
		status := s.Accessibility.Status(context.Background())
		if !status.Available {
			return nil, "", &Error{Code: UI_TREE_ACCESSIBILITY_DISABLED, Message: "accessibility source unavailable: " + status.Reason}
		}
		return s.Accessibility, SourceTypeAccessibility, nil

	case SourceRoot:
		if !allowRootFallback {
			return nil, "", &Error{Code: UI_TREE_INVALID_SOURCE, Message: "root source requires explicit permission"}
		}
		if s.Root == nil {
			return nil, "", &Error{Code: UI_TREE_ROOT_UNAVAILABLE, Message: "root source not configured"}
		}
		status := s.Root.Status(context.Background())
		if !status.Available {
			return nil, "", &Error{Code: UI_TREE_ROOT_UNAVAILABLE, Message: "root source unavailable: " + status.Reason}
		}
		return s.Root, SourceTypeRoot, nil

	case SourceADB:
		if s.ADB == nil {
			return nil, "", &Error{Code: UI_TREE_ADB_UNAVAILABLE, Message: "adb source not configured"}
		}
		status := s.ADB.Status(context.Background())
		if !status.Available {
			return nil, "", &Error{Code: UI_TREE_ADB_UNAVAILABLE, Message: "adb source unavailable: " + status.Reason}
		}
		return s.ADB, SourceTypeADB, nil

	default:
		return nil, "", &Error{Code: UI_TREE_INVALID_SOURCE, Message: "unknown source: " + source}
	}
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
