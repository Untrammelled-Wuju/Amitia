package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	ConfigSchemaVersion = 1
	PipelineVersion     = "image-pipeline/1"
)

type ProcessingConfigSnapshot struct {
	SchemaVersion     int                    `json:"schemaVersion"`
	Canvas            CanvasPolicy           `json:"canvas"`
	Decode            DecodePolicy           `json:"decode"`
	Split             SplitPolicy            `json:"split"`
	Background        BackgroundPolicy       `json:"background"`
	Subject           SubjectPolicy          `json:"subject"`
	Scale             ScalePolicy            `json:"scale"`
	Anchor            AnchorPolicy           `json:"anchor"`
	Alignment         AlignmentPolicy        `json:"alignment"`
	Encoding          EncodingPolicy         `json:"encoding"`
	AlgorithmVersions map[string]string      `json:"algorithmVersions"`
	ConfigHash        string                 `json:"configHash"`
}

type CanvasPolicy struct {
	OutputWidth                int     `json:"outputWidth"`
	OutputHeight               int     `json:"outputHeight"`
	SafeMarginTop              int     `json:"safeMarginTop"`
	SafeMarginRight            int     `json:"safeMarginRight"`
	SafeMarginBottom           int     `json:"safeMarginBottom"`
	SafeMarginLeft             int     `json:"safeMarginLeft"`
	TargetCharacterHeightRatio float64 `json:"targetCharacterHeightRatio"`
	MaxCharacterWidthRatio     float64 `json:"maxCharacterWidthRatio"`
	TransparentBackground      bool    `json:"transparentBackground"`
}

type DecodePolicy struct {
	MaxInputBytes    int64    `json:"maxInputBytes"`
	MaxPixels        int64    `json:"maxPixels"`
	MaxDimension     int      `json:"maxDimension"`
	AllowedMIMEs     []string `json:"allowedMIMEs"`
	ApplyEXIF        bool     `json:"applyEXIF"`
	ColorSpacePolicy string   `json:"colorSpacePolicy"`
}

type SplitPolicy struct {
	AllowSizeTolerance  bool `json:"allowSizeTolerance"`
	MaxTolerancePixels  int  `json:"maxTolerancePixels"`
	EmptyCellHandling   string `json:"emptyCellHandling"`
	CropBoundaryPolicy  string `json:"cropBoundaryPolicy"`
}

type BackgroundPolicy struct {
	ProviderName       string        `json:"providerName"`
	Mode               string        `json:"mode"`
	CapabilitiesVersion string       `json:"capabilitiesVersion"`
	FallbackPolicy     string        `json:"fallbackPolicy"`
	Timeout            time.Duration `json:"timeout"`
	MaxRetries         int           `json:"maxRetries"`
	KeepMask           bool          `json:"keepMask"`
}

type SubjectPolicy struct {
	AlphaThreshold      int     `json:"alphaThreshold"`
	MinComponentArea    float64 `json:"minComponentArea"`
	SatelliteAreaRatio  float64 `json:"satelliteAreaRatio"`
	MaxComponents       int     `json:"maxComponents"`
}

type ScalePolicy struct {
	BaselineSource      string  `json:"baselineSource"`
	StatMethod          string  `json:"statMethod"`
	ActionScaleMaxDelta float64 `json:"actionScaleMaxDelta"`
	Resampler           string  `json:"resampler"`
}

type AnchorPolicy struct {
	Source          string  `json:"source"`
	Mode            string  `json:"mode"`
	TargetX         float64 `json:"targetX"`
	TargetY         float64 `json:"targetY"`
	EstimationStrategy string `json:"estimationStrategy"`
}

type AlignmentPolicy struct {
	StabilizationAxes   string  `json:"stabilizationAxes"`
	AllowMotionX        bool    `json:"allowMotionX"`
	AllowMotionY        bool    `json:"allowMotionY"`
	ReferenceStrategy   string  `json:"referenceStrategy"`
	MaxCorrectionX      float64 `json:"maxCorrectionX"`
	MaxCorrectionY      float64 `json:"maxCorrectionY"`
}

type EncodingPolicy struct {
	Format           string `json:"format"`
	CompressionLevel int    `json:"compressionLevel"`
	ColorModel       string `json:"colorModel"`
	WriteMask        bool   `json:"writeMask"`
}

func NewDefaultConfigSnapshot(outputWidth, outputHeight int, targetHeightRatio float64, anchorMode, backgroundMode, backgroundProvider, resampleMode string) *ProcessingConfigSnapshot {
	if outputWidth <= 0 {
		outputWidth = 512
	}
	if outputHeight <= 0 {
		outputHeight = 512
	}
	if targetHeightRatio <= 0 || targetHeightRatio > 1 {
		targetHeightRatio = 0.8
	}
	if anchorMode == "" {
		anchorMode = "feet_center"
	}
	if backgroundMode == "" {
		backgroundMode = "remove_background"
	}
	if backgroundProvider == "" {
		backgroundProvider = "local-color-key"
	}
	if resampleMode == "" {
		resampleMode = "auto"
	}

	cfg := &ProcessingConfigSnapshot{
		SchemaVersion: ConfigSchemaVersion,
		Canvas: CanvasPolicy{
			OutputWidth:                outputWidth,
			OutputHeight:               outputHeight,
			SafeMarginTop:              8,
			SafeMarginRight:            8,
			SafeMarginBottom:           8,
			SafeMarginLeft:             8,
			TargetCharacterHeightRatio: targetHeightRatio,
			MaxCharacterWidthRatio:     0.9,
			TransparentBackground:      true,
		},
		Decode: DecodePolicy{
			MaxInputBytes:    64 * 1024 * 1024,
			MaxPixels:        64 * 1024 * 1024,
			MaxDimension:     16384,
			AllowedMIMEs:     []string{"image/png", "image/jpeg", "image/webp"},
			ApplyEXIF:        true,
			ColorSpacePolicy: "srgb",
		},
		Split: SplitPolicy{
			AllowSizeTolerance: true,
			MaxTolerancePixels: 2,
			EmptyCellHandling:  "skip",
			CropBoundaryPolicy: "half_open",
		},
		Background: BackgroundPolicy{
			ProviderName:        backgroundProvider,
			Mode:                backgroundMode,
			CapabilitiesVersion: "1",
			FallbackPolicy:      "none",
			Timeout:             60 * time.Second,
			MaxRetries:          2,
			KeepMask:            true,
		},
		Subject: SubjectPolicy{
			AlphaThreshold:     10,
			MinComponentArea:   0.005,
			SatelliteAreaRatio: 0.05,
			MaxComponents:      10,
		},
		Scale: ScalePolicy{
			BaselineSource:      "default_idle",
			StatMethod:          "median",
			ActionScaleMaxDelta: 0.15,
			Resampler:           resampleMode,
		},
		Anchor: AnchorPolicy{
			Source:             "action_spec",
			Mode:               anchorMode,
			TargetX:            0.5,
			TargetY:            0.92,
			EstimationStrategy: "auto",
		},
		Alignment: AlignmentPolicy{
			StabilizationAxes: "xy",
			AllowMotionX:      false,
			AllowMotionY:      false,
			ReferenceStrategy: "median_anchor",
			MaxCorrectionX:    0.05,
			MaxCorrectionY:    0.05,
		},
		Encoding: EncodingPolicy{
			Format:           "png",
			CompressionLevel: 6,
			ColorModel:       "rgba",
			WriteMask:        true,
		},
		AlgorithmVersions: map[string]string{
			"decoder":    "bounded-v1",
			"splitter":   "layout-v1",
			"background": "local-color-key-v1",
			"geometry":   "mask-analyzer-v1",
			"scale":      "baseline-median-v1",
			"anchor":     "mask-centroid-v1",
			"canvas":     "source-anchor-v1",
			"alignment":  "median-stabilize-v1",
			"encoder":    "png-v1",
		},
	}

	cfg.ConfigHash = cfg.ComputeHash()
	return cfg
}

func (c *ProcessingConfigSnapshot) ComputeHash() string {
	clone := *c
	clone.ConfigHash = ""
	data, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (c *ProcessingConfigSnapshot) Validate() error {
	if c.Canvas.OutputWidth <= 0 || c.Canvas.OutputHeight <= 0 {
		return ErrInvalidConfig("canvas dimensions must be positive")
	}
	if c.Canvas.TargetCharacterHeightRatio <= 0 || c.Canvas.TargetCharacterHeightRatio > 1 {
		return ErrInvalidConfig("target character height ratio must be in (0, 1]")
	}
	if c.Decode.MaxInputBytes <= 0 || c.Decode.MaxPixels <= 0 {
		return ErrInvalidConfig("decode limits must be positive")
	}
	if c.Encoding.Format != "png" {
		return ErrInvalidConfig("only png format is currently supported, got: " + c.Encoding.Format)
	}
	return nil
}
