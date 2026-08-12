package clipboard

import "fmt"

const (
	MaxInlineTextBytes            = 64 * 1024
	MaxClipboardReadBytes         = 64 * 1024
	MaxMaterializedClipboardBytes = 10 * 1024 * 1024
	MaxURLBytes                   = 16 * 1024
	DefaultMaxItems               = 1
	MaxItemsLimit                 = 16
	MaxWriteItems                 = 16
	MaxExpirationSeconds          = 3600
	DefaultExpirationSensitive    = 120
	DefaultExpirationSecret       = 60
)

var AllowedPatternKinds = []PatternKind{
	PatternProbableWebURL,
	PatternProbableWebSearch,
	PatternNumber,
	PatternEmailAddress,
	PatternPhoneNumber,
	PatternPostalAddress,
	PatternCreditCardNumber,
}

var AllowedReadTypes = []ContentType{
	ContentTypeTextPlain,
	ContentTypeTextHTML,
	ContentTypeTextRTF,
	ContentTypeTextURI,
	ContentTypeImagePNG,
	ContentTypeImageJPEG,
	ContentTypeImageGIF,
	ContentTypeImageWEBP,
	ContentTypeImageHEIC,
	ContentTypeFileURL,
}

func IsValidPatternKind(p PatternKind) bool {
	for _, k := range AllowedPatternKinds {
		if k == p {
			return true
		}
	}
	return false
}

func IsValidReadType(t ContentType) bool {
	for _, ct := range AllowedReadTypes {
		if ct == t {
			return true
		}
	}
	return false
}

func ClampMaxItems(n int) int {
	if n <= 0 {
		return DefaultMaxItems
	}
	if n > MaxItemsLimit {
		return MaxItemsLimit
	}
	return n
}

func ClampMaxBytes(n int64) int64 {
	if n <= 0 {
		return MaxClipboardReadBytes
	}
	if n > MaxMaterializedClipboardBytes {
		return MaxMaterializedClipboardBytes
	}
	return n
}

func ClampExpirationSeconds(n *int) *int {
	if n == nil {
		return nil
	}
	if *n <= 0 {
		return nil
	}
	if *n > MaxExpirationSeconds {
		v := MaxExpirationSeconds
		return &v
	}
	return n
}

func ValidateReadRequest(req ClipboardReadRequest) error {
	for _, t := range req.PreferredTypes {
		if !IsValidReadType(ContentType(t)) {
			return fmt.Errorf("%v: unsupported read type %q", ErrTypeUnsupported, t)
		}
	}
	for _, idx := range req.ItemIndexes {
		if idx < 0 {
			return fmt.Errorf("%v: negative item index %d", ErrItemNotFound, idx)
		}
	}
	if req.MaxItems > MaxItemsLimit {
		return fmt.Errorf("%v: maxItems %d exceeds limit %d", ErrItemNotFound, req.MaxItems, MaxItemsLimit)
	}
	if req.MaxBytes > MaxMaterializedClipboardBytes {
		return fmt.Errorf("%v: maxBytes exceeds limit", ErrContentTooLarge)
	}
	return nil
}

func ValidateWriteRequest(req ClipboardWriteRequest) error {
	if len(req.Items) == 0 {
		return fmt.Errorf("%v: at least one item required", ErrWriteValueRequired)
	}
	if len(req.Items) > MaxWriteItems {
		return fmt.Errorf("%v: too many items (%d > %d)", ErrWriteItemLimitExceeded, len(req.Items), MaxWriteItems)
	}
	for i, item := range req.Items {
		if !IsValidReadType(ContentType(item.Type)) {
			return fmt.Errorf("%v: unsupported write type %q at index %d", ErrWriteTypeNotAllowed, item.Type, i)
		}
		if ContentType(item.Type) == ContentTypeTextURI && len(item.URL) > MaxURLBytes {
			return fmt.Errorf("%v: URL exceeds maximum length", ErrURLTooLong)
		}
	}
	return nil
}

func ValidatePatterns(patterns []PatternKind) error {
	for _, p := range patterns {
		if !IsValidPatternKind(p) {
			return fmt.Errorf("%v: unsupported pattern %q", ErrDetectionUnsupported, p)
		}
	}
	return nil
}
