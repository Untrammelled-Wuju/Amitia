package prompt

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/pkg/util"
)

func TestBaseIdentitySectionIncludesAmitiaMessageBreak(t *testing.T) {
	if !strings.Contains(BaseIdentitySection(), util.AmitiaMessageBreak) {
		t.Fatalf("base identity section must document %s", util.AmitiaMessageBreak)
	}
}
