package mediaread

type ImageInfo struct {
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType"`
	Format      string `json:"format"`
	SizeBytes   int64  `json:"sizeBytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Orientation int    `json:"orientation"`
	HasAlpha    bool   `json:"hasAlpha"`
	Animated    bool   `json:"animated"`
	Source      string `json:"source"`
}

type DecodeOptions struct {
	MaxWidth             int   `json:"maxWidth,omitempty"`
	MaxHeight            int   `json:"maxHeight,omitempty"`
	MaxPixels            int64 `json:"maxPixels,omitempty"`
	NormalizeOrientation bool  `json:"normalizeOrientation,omitempty"`
	StripMetadata        bool  `json:"stripMetadata,omitempty"`
}

type NormalizedImage struct {
	ResourceURI  string `json:"resourceUri"`
	MIMEType     string `json:"mimeType"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	SizeBytes    int64  `json:"sizeBytes"`
	Normalized   bool   `json:"normalized"`
	SourceURI    string `json:"sourceUri,omitempty"`
}

type ResolvedResource struct {
	URI       string
	LocalPath string
	MIMEType  string
	SizeBytes int64
	Source    string
}

func (r ResolvedResource) IsValid() bool {
	return r.URI != "" && r.LocalPath != ""
}

const (
	SourceCamera     = "camera"
	SourceScreenshot = "screenshot"
	SourceWorkspace  = "workspace"
	SourceAttachment = "attachment"
	SourceTemp       = "temp"
	SourceCache      = "cache"
	SourceUnknown    = "unknown"
)

const (
	FormatJPEG = "jpeg"
	FormatPNG  = "png"
	FormatWebP = "webp"
	FormatGIF  = "gif"
	FormatBMP  = "bmp"
	FormatHEIC = "heic"
	FormatHEIF = "heif"
)

var formatToExt = map[string]string{
	FormatJPEG: ".jpg",
	FormatPNG:  ".png",
	FormatWebP: ".webp",
	FormatGIF:  ".gif",
	FormatBMP:  ".bmp",
	FormatHEIC: ".heic",
	FormatHEIF: ".heif",
}

func FormatToExt(format string) string {
	if ext, ok := formatToExt[format]; ok {
		return ext
	}
	return ".bin"
}

func ExtToFormat(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return FormatJPEG
	case ".png":
		return FormatPNG
	case ".webp":
		return FormatWebP
	case ".gif":
		return FormatGIF
	case ".bmp":
		return FormatBMP
	case ".heic":
		return FormatHEIC
	case ".heif":
		return FormatHEIF
	}
	return ""
}

func FormatToMIME(format string) string {
	switch format {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatWebP:
		return "image/webp"
	case FormatGIF:
		return "image/gif"
	case FormatBMP:
		return "image/bmp"
	case FormatHEIC, FormatHEIF:
		return "image/heic"
	}
	return "application/octet-stream"
}
