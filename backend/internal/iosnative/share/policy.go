package share

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	MaxResourcesCount        = 10
	MaxSingleResourceBytes   = 100 * 1024 * 1024
	MaxTotalBytes            = 250 * 1024 * 1024
	MaxShareTextBytes        = 1 * 1024 * 1024
	MaxSubjectBytes          = 8 * 1024
	ShareTitleMaxBytes       = 256
	MaxPreviewTitleChars     = 256
	MaxPreviewSubtitleChars  = 256
	SharedShareDefaultTimeoutSec = 300
	MaxIntakeTextBytes       = MaxShareTextBytes
	MaxIntakeResourcesCount  = MaxResourcesCount
	MaxIntakeTotalBytes      = MaxTotalBytes
	StagingMaxStaleAgeHours  = 24
	StagingConsumedRetentionHours = 168
	URLSchemeHTTP            = "http"
	URLSchemeHTTPS           = "https"
)

var AllowedURLSchemes = []string{URLSchemeHTTP, URLSchemeHTTPS}

func IsValidURLScheme(scheme string) bool {
	for _, s := range AllowedURLSchemes {
		if s == scheme {
			return true
		}
	}
	return false
}

func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%v: %w", ErrShareURLInvalid, err)
	}
	if !IsValidURLScheme(strings.ToLower(u.Scheme)) {
		return fmt.Errorf("%v: scheme %q not allowed", ErrShareURLSchemeNotAllowed, u.Scheme)
	}
	return nil
}

func ValidateSendRequest(req IOSShareSendRequest) error {
	textBytes := len([]byte(req.Text))
	if textBytes > MaxShareTextBytes {
		return fmt.Errorf("%v: text length %d exceeds max %d", ErrShareTextTooLong, textBytes, MaxShareTextBytes)
	}

	subjectBytes := len([]byte(req.Subject))
	if subjectBytes > MaxSubjectBytes {
		return fmt.Errorf("%v: subject length %d exceeds max %d", ErrShareSubjectTooLong, subjectBytes, MaxSubjectBytes)
	}

	shareTitleBytes := len([]byte(req.ShareTitle))
	if shareTitleBytes > ShareTitleMaxBytes {
		return fmt.Errorf("%v: shareTitle length %d exceeds max %d", ErrShareShareTitleTooLong, shareTitleBytes, ShareTitleMaxBytes)
	}

	if len(req.Resources) > MaxResourcesCount {
		return fmt.Errorf("%v: resource count %d exceeds max %d", ErrShareTooManyResources, len(req.Resources), MaxResourcesCount)
	}

	if req.URL != "" {
		if err := ValidateURL(req.URL); err != nil {
			return err
		}
	}

	if req.Preview != nil {
		if len(req.Preview.Title) > MaxPreviewTitleChars {
			return fmt.Errorf("%v: preview title exceeds max %d chars", ErrSharePreviewTitleTooLong, MaxPreviewTitleChars)
		}
		if len(req.Preview.Subtitle) > MaxPreviewSubtitleChars {
			return fmt.Errorf("%v: preview subtitle exceeds max %d chars", ErrSharePreviewSubtitleTooLong, MaxPreviewSubtitleChars)
		}
	}

	return nil
}

func ValidateIncomingItem(item IOSIncomingShareItem) error {
	if item.ItemID == "" {
		return fmt.Errorf("%v: itemId is required", ErrShareStagingInvalid)
	}
	if item.RelativePath != "" {
		if strings.Contains(item.RelativePath, "..") || strings.HasPrefix(item.RelativePath, "/") {
			return fmt.Errorf("%v: relativePath contains escape sequence", ErrShareStagingPathEscape)
		}
	}
	return nil
}

func ValidateIncomingManifest(manifest IOSIncomingShareManifest) error {
	if manifest.ShareID == "" {
		return fmt.Errorf("%v: shareId is required", ErrShareStagingInvalid)
	}
	if !manifest.Complete {
		return fmt.Errorf("%v", ErrShareStagingNotCommitted)
	}
	if len(manifest.Items) > MaxIntakeResourcesCount {
		return fmt.Errorf("%v: item count %d exceeds max %d", ErrShareStagingTooManyResources, len(manifest.Items), MaxIntakeResourcesCount)
	}
	if manifest.TotalBytes > MaxIntakeTotalBytes {
		return fmt.Errorf("%v: total bytes %d exceeds max %d", ErrShareStagingTooLarge, manifest.TotalBytes, MaxIntakeTotalBytes)
	}
	if len(manifest.Text) > MaxIntakeTextBytes {
		return fmt.Errorf("%v: text length %d exceeds max %d", ErrShareStagingTextTooLong, len(manifest.Text), MaxIntakeTextBytes)
	}
	for _, item := range manifest.Items {
		if err := ValidateIncomingItem(item); err != nil {
			return err
		}
	}
	return nil
}

func ClampMaxStaleAgeHours(hours int) int {
	if hours <= 0 {
		return StagingMaxStaleAgeHours
	}
	return hours
}
