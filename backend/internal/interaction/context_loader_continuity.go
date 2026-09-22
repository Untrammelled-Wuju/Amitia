package interaction

import (
	"context"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/continuity"
)

type ContinuityContextLoader struct {
	repo *continuity.Repository
}

func NewContinuityContextLoader(repo *continuity.Repository) *ContinuityContextLoader {
	return &ContinuityContextLoader{repo: repo}
}

func (l *ContinuityContextLoader) Name() string           { return "continuity" }
func (l *ContinuityContextLoader) IsRequired() bool       { return false }
func (l *ContinuityContextLoader) Timeout() time.Duration { return 350 * time.Millisecond }
func (l *ContinuityContextLoader) CacheKey(scope InteractionScope, version string) string {
	return "continuity|" + strings.TrimSpace(scope.SpaceID) + "|" + strings.TrimSpace(scope.ThreadID) + "|" + version
}

func (l *ContinuityContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	_ = ctx
	if l == nil || l.repo == nil || strings.TrimSpace(scope.ThreadID) == "" {
		return FieldUnavailable[any]("continuity-runtime"), nil
	}
	value, err := continuity.BuildContext(l.repo, scope.ThreadID, scope.SpaceID)
	if err != nil {
		return FieldUnavailable[any]("continuity-runtime"), err
	}
	if strings.TrimSpace(value.ThreadID) == "" {
		return FieldUnavailable[any]("continuity-runtime"), nil
	}
	return SnapshotField[any]{Value: value, Source: "continuity-runtime", Status: LoadStatusReady, Version: version}, nil
}
