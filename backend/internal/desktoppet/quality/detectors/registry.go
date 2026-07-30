// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package detectors

import "github.com/u-ai/backend/internal/desktoppet/quality"

func NewDefaultDetectors() []quality.Detector {
	return []quality.Detector{
		NewIntegrityDetector(),
		NewSubjectDetector(),
		NewBackgroundDetector(),
		NewEdgeDetector(),
		NewStabilityDetector(),
		NewIdentityDetector(),
		NewMotionDetector(),
		NewDuplicateDetector(),
		NewLoopDetector(),
		NewColorDetector(),
	}
}
