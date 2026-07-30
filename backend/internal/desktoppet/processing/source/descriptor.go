package source

type SourceKind string

const (
	SourceSpriteSheet SourceKind = "sprite_sheet"
	SourceKeyframe    SourceKind = "keyframe"
	SourceSingleFrame SourceKind = "single_frame"
	SourceLegacyFrame SourceKind = "legacy_frame"
)

type ProcessingSourceDescriptor struct {
	SourceKind       SourceKind
	ActionKey        string
	ActionSpecRef    string
	GenerationMode   string
	SourceAttemptID  string
	CandidateIndex   int
	Artifact         GenerationArtifactDescriptor
	Frames           []ProcessingSourceFrame
	LayoutHash       string
	SourceConfigHash string
}

type ProcessingSourceFrame struct {
	LogicalFrameIndex int
	SourceArtifactID  string
	SourceCellIndex   *int
	RelativePath      string
	CropRect          PixelRect
	ExpectedHash      string
	ExpectedMIME      string
	ExpectedWidth     int
	ExpectedHeight    int
}

type GenerationArtifactDescriptor struct {
	ArtifactID       string
	GenerationTaskID string
	ActionID         string
	AttemptID        string
	GenerationMode   string
	CandidateIndex   int
	Primary          bool
	RelativePath     string
	MIMEType         string
	Width            int
	Height           int
	ByteSize         int64
	ContentHash      string
	Layout           *SpriteSheetLayoutSnapshot
	LogicalFrames    []LogicalFrameDescriptor
	CreatedAt        string
}

type SpriteSheetLayoutSnapshot struct {
	SchemaVersion    int
	Rows             int
	Columns          int
	CellWidth        int
	CellHeight       int
	MarginTop        int
	MarginRight      int
	MarginBottom     int
	MarginLeft       int
	GapX             int
	GapY             int
	ReadingOrder     string
	ExpectedWidth    int
	ExpectedHeight   int
	EmptyCellIndexes []int
	Cells            []SpriteSheetCell
}

type SpriteSheetCell struct {
	CellIndex  int
	Row        int
	Column     int
	FrameIndex *int
	X          int
	Y          int
	Width      int
	Height     int
	Empty      bool
}

type LogicalFrameDescriptor struct {
	FrameIndex    int
	CellIndex     *int
	KeyframeIndex *int
}

type PixelRect struct {
	MinX, MinY, MaxX, MaxY int
}

func (r PixelRect) Width() int {
	return r.MaxX - r.MinX
}

func (r PixelRect) Height() int {
	return r.MaxY - r.MinY
}
