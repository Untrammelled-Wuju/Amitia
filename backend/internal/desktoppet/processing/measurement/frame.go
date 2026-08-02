package measurement

import "encoding/json"

type FrameMeasurementData struct {
	FrameIndex     int                    `json:"frameIndex"`
	SubjectBox     SubjectBoxData         `json:"subjectBox"`
	SourceAnchor   AnchorData             `json:"sourceAnchor"`
	TargetAnchor   AnchorData             `json:"targetAnchor"`
	AlphaCoverage  float64                `json:"alphaCoverage"`
	ComponentCount int                    `json:"componentCount"`
	EdgeContact    EdgeContactData        `json:"edgeContact"`
	Clipping       ClippingData           `json:"clipping"`
	Trajectory     TrajectoryData         `json:"trajectory"`
	StageMetrics   map[string]StageMetric `json:"stageMetrics,omitempty"`
}

type SubjectBoxData struct {
	MinX  int    `json:"minX"`
	MinY  int    `json:"minY"`
	MaxX  int    `json:"maxX"`
	MaxY  int    `json:"maxY"`
	Space string `json:"space"`
}

type AnchorData struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Space      string  `json:"space"`
	Mode       string  `json:"mode"`
	Confidence float64 `json:"confidence"`
	Estimated  bool    `json:"estimated"`
}

type EdgeContactData struct {
	Top    bool `json:"top"`
	Bottom bool `json:"bottom"`
	Left   bool `json:"left"`
	Right  bool `json:"right"`
	Count  int  `json:"count"`
}

type ClippingData struct {
	TopPixels    int    `json:"topPixels"`
	BottomPixels int    `json:"bottomPixels"`
	LeftPixels   int    `json:"leftPixels"`
	RightPixels  int    `json:"rightPixels"`
	TotalPixels  int    `json:"totalPixels"`
	Sides        string `json:"sides"`
}

type TrajectoryData struct {
	OriginalX   float64 `json:"originalX"`
	OriginalY   float64 `json:"originalY"`
	CorrectedX  float64 `json:"correctedX"`
	CorrectedY  float64 `json:"correctedY"`
	CorrectionX float64 `json:"correctionX"`
	CorrectionY float64 `json:"correctionY"`
	Clamped     bool    `json:"clamped"`
}

type StageMetric struct {
	DurationMs      int64  `json:"durationMs"`
	PixelsProcessed int64  `json:"pixelsProcessed"`
	CacheHit        bool   `json:"cacheHit"`
	ErrorCode       string `json:"errorCode,omitempty"`
}

type SequenceMeasurement struct {
	ActionKey         string                 `json:"actionKey"`
	FrameCount        int                    `json:"frameCount"`
	FrameMeasurements []FrameMeasurementData `json:"frameMeasurements"`
	ScaleResult       ScaleResultData        `json:"scaleResult"`
	ReferenceFrame    int                    `json:"referenceFrame"`
	ReferenceStrategy string                 `json:"referenceStrategy"`
	ProcessingReport  ProcessingReport       `json:"processingReport"`
}

type ScaleResultData struct {
	Scale           float64 `json:"scale"`
	BaseScale       float64 `json:"baseScale"`
	ClampedScale    float64 `json:"clampedScale"`
	ClampReason     string  `json:"clampReason,omitempty"`
	ReferenceHeight float64 `json:"referenceHeight"`
	ReferenceWidth  float64 `json:"referenceWidth"`
}

type ProcessingReport struct {
	PipelineVersion string        `json:"pipelineVersion"`
	ConfigHash      string        `json:"configHash"`
	TotalDurationMs int64         `json:"totalDurationMs"`
	ProviderUsed    string        `json:"providerUsed"`
	Degraded        bool          `json:"degraded"`
	DegradedReason  string        `json:"degradedReason,omitempty"`
	Stages          []StageReport `json:"stages"`
}

type StageReport struct {
	Name          string `json:"name"`
	DurationMs    int64  `json:"durationMs"`
	Status        string `json:"status"`
	ErrorCode     string `json:"errorCode,omitempty"`
	CacheHit      bool   `json:"cacheHit"`
	ArtifactCount int    `json:"artifactCount"`
}

func NewFrameMeasurement(frameIndex int) *FrameMeasurementData {
	return &FrameMeasurementData{
		FrameIndex:   frameIndex,
		StageMetrics: make(map[string]StageMetric),
	}
}

func (m *FrameMeasurementData) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
