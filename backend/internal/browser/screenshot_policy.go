package browser

const (
	DefaultScreenshotFormat = ScreenshotFormatPNG

	MaxScreenshotPixels = 40_000_000
	MaxScreenshotBytes  = 64 * 1024 * 1024
	MaxDimension        = 16384

	MinQuality = 0
	MaxQuality = 100
)

type ScreenshotPolicy struct {
	MaxPixels       int64
	MaxBytes        int64
	MaxDimension    int
	AllowFullPage   bool
	DefaultFormat   string
	AllowedFormats  []string
	StagingRootPath string
}

func DefaultScreenshotPolicy() ScreenshotPolicy {
	return ScreenshotPolicy{
		MaxPixels:      MaxScreenshotPixels,
		MaxBytes:       MaxScreenshotBytes,
		MaxDimension:   MaxDimension,
		AllowFullPage:  true,
		DefaultFormat:  DefaultScreenshotFormat,
		AllowedFormats: []string{ScreenshotFormatPNG, ScreenshotFormatJPEG, ScreenshotFormatWebP},
	}
}

func IsValidScreenshotFormat(format string) bool {
	switch format {
	case ScreenshotFormatPNG, ScreenshotFormatJPEG, ScreenshotFormatWebP:
		return true
	default:
		return false
	}
}

func IsValidScreenshotQuality(format string, quality int) bool {
	if quality < MinQuality || quality > MaxQuality {
		return false
	}
	if format == ScreenshotFormatPNG {
		return quality == 0
	}
	return true
}
