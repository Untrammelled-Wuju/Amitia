package generationlayout

type ReadingOrder string

const (
	ReadingRowMajorLeftToRight ReadingOrder = "row_major_left_to_right_top_to_bottom"
	ReadingRowMajorRightToLeft ReadingOrder = "row_major_right_to_left_top_to_bottom"
	ReadingColumnMajor         ReadingOrder = "column_major_top_to_bottom"
)

type Cell struct {
	Row        int
	Column     int
	FrameIndex int
	IsEmpty    bool
}

type SheetLayout struct {
	Rows         int
	Columns      int
	CellWidth    int
	CellHeight   int
	MarginX      int
	MarginY      int
	GapX         int
	GapY         int
	SheetWidth   int
	SheetHeight  int
	ReadingOrder ReadingOrder
	Cells        []Cell
	EmptyCells   []int
	TotalCells   int
	UsedCells    int
	FrameCount   int
}

type SegmentLayout struct {
	SegmentIndex    int
	SheetLayout     SheetLayout
	FrameStartIndex int
	FrameEndIndex   int
	FrameCount      int
}

type LayoutResult struct {
	Segments       []SegmentLayout
	TotalSegments  int
	TotalSheets    int
	TotalCells     int
	TotalUsedCells int
	FrameCount     int
	CellWidth      int
	CellHeight     int
	SheetWidth     int
	SheetHeight    int
}
