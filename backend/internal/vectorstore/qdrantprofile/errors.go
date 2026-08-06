// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import "fmt"

var (
	ErrProfileRequired              = fmt.Errorf("qdrantprofile: profile required")
	ErrUnknownProfile               = fmt.Errorf("qdrantprofile: unknown profile")
	ErrInvalidProfileSettings       = fmt.Errorf("qdrantprofile: invalid profile settings")
	ErrRuntimeDescriptorUnavailable = fmt.Errorf("qdrantprofile: runtime descriptor unavailable")
	ErrRuntimeClassificationFailed  = fmt.Errorf("qdrantprofile: runtime classification failed")
	ErrQdrantProcessUnsupported     = fmt.Errorf("qdrantprofile: qdrant process unsupported")
	ErrUnsupportedGuestPlatform     = fmt.Errorf("qdrantprofile: unsupported guest platform")
	ErrUnsupportedGuestArchitecture = fmt.Errorf("qdrantprofile: unsupported guest architecture")
	ErrEnvironmentSanitizationFailed = fmt.Errorf("qdrantprofile: environment sanitization failed")
)
