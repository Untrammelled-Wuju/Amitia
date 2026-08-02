package imageprovider

type GenerationMode string

const (
	ModeSpriteSheet GenerationMode = "sprite_sheet"
	ModeKeyframe    GenerationMode = "keyframe"
	ModeSingleFrame GenerationMode = "single_frame"
	ModeLegacyFrame GenerationMode = "legacy_frame"
)

type ImageSize struct {
	Width  int
	Height int
}

type DimensionRule struct {
	AllowedSizes   []ImageSize
	MinWidth       int
	MaxWidth       int
	MinHeight      int
	MaxHeight      int
	MaxTotalPixels int64
	AllowedRatios  []string
	WidthMultiple  int
	HeightMultiple int
}

type ProviderCapabilities struct {
	SchemaVersion           int
	Provider                string
	Model                   string
	SupportedModes          []GenerationMode
	SupportsReferenceImage  bool
	SupportsMultipleImages  bool
	SupportsNegativePrompt  bool
	SupportsSeed            bool
	SupportsAsyncOperation  bool
	SupportsCancellation    bool
	SupportsIdempotencyKey  bool
	SupportsTransparentHint bool
	SupportsMultipleSheets  bool
	MaxReferenceImages      int
	MaxOutputImages         int
	MaxPromptCharacters     int
	MaxInputImageBytes      int64
	SupportedInputMIMEs     []string
	SupportedOutputMIMEs    []string
	Dimensions              DimensionRule
	RecommendedGridColumns  []int
	RecommendedGridRows     []int
	CapabilityVersion       string
	RetrievedAt             string
	ExpiresAt               string
}

func (c ProviderCapabilities) SupportsMode(mode GenerationMode) bool {
	for _, m := range c.SupportedModes {
		if m == mode {
			return true
		}
	}
	return false
}

func (c ProviderCapabilities) SupportsSize(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	if c.Dimensions.MinWidth > 0 && width < c.Dimensions.MinWidth {
		return false
	}
	if c.Dimensions.MaxWidth > 0 && width > c.Dimensions.MaxWidth {
		return false
	}
	if c.Dimensions.MinHeight > 0 && height < c.Dimensions.MinHeight {
		return false
	}
	if c.Dimensions.MaxHeight > 0 && height > c.Dimensions.MaxHeight {
		return false
	}
	if c.Dimensions.MaxTotalPixels > 0 {
		total := int64(width) * int64(height)
		if total > c.Dimensions.MaxTotalPixels {
			return false
		}
	}
	if c.Dimensions.WidthMultiple > 0 && width%c.Dimensions.WidthMultiple != 0 {
		return false
	}
	if c.Dimensions.HeightMultiple > 0 && height%c.Dimensions.HeightMultiple != 0 {
		return false
	}
	if len(c.Dimensions.AllowedSizes) > 0 {
		for _, s := range c.Dimensions.AllowedSizes {
			if s.Width == width && s.Height == height {
				return true
			}
		}
		return false
	}
	return true
}

func (c ProviderCapabilities) SupportsInputMIME(mime string) bool {
	if len(c.SupportedInputMIMEs) == 0 {
		return true
	}
	for _, m := range c.SupportedInputMIMEs {
		if m == mime {
			return true
		}
	}
	return false
}

func (c ProviderCapabilities) SupportsOutputMIME(mime string) bool {
	if len(c.SupportedOutputMIMEs) == 0 {
		return true
	}
	for _, m := range c.SupportedOutputMIMEs {
		if m == mime {
			return true
		}
	}
	return false
}

func (c ProviderCapabilities) ClampOutputCount(requested int) int {
	if requested <= 0 {
		return 1
	}
	if c.MaxOutputImages > 0 && requested > c.MaxOutputImages {
		return c.MaxOutputImages
	}
	return requested
}

func (c ProviderCapabilities) ClampReferenceImages(requested int) int {
	if requested <= 0 {
		return 0
	}
	if c.MaxReferenceImages > 0 && requested > c.MaxReferenceImages {
		return c.MaxReferenceImages
	}
	return requested
}

type CapabilitySnapshot struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Provider      string               `json:"provider"`
	Model         string               `json:"model"`
	Capabilities  ProviderCapabilities `json:"capabilities"`
	Hash          string               `json:"hash"`
	CreatedAt     string               `json:"createdAt"`
}
