package generationlayout

const (
	DefaultCellWidth  = 512
	DefaultCellHeight = 512
	DefaultMargin     = 0
	DefaultGap        = 0
)

type GridPreset struct {
	FrameCount int
	Rows       int
	Columns    int
	Name       string
}

var gridPresets = []GridPreset{
	{FrameCount: 4, Rows: 2, Columns: 2, Name: "2x2"},
	{FrameCount: 6, Rows: 2, Columns: 3, Name: "3x2"},
	{FrameCount: 8, Rows: 2, Columns: 4, Name: "4x2"},
	{FrameCount: 10, Rows: 2, Columns: 5, Name: "5x2"},
	{FrameCount: 12, Rows: 3, Columns: 4, Name: "4x3"},
}

func Presets() []GridPreset {
	return gridPresets
}

func RecommendGrid(frameCount int) (rows, columns int) {
	if frameCount <= 0 {
		return 1, 1
	}
	for _, p := range gridPresets {
		if p.FrameCount == frameCount {
			return p.Rows, p.Columns
		}
	}
	columns = 4
	rows = (frameCount + columns - 1) / columns
	return rows, columns
}

func RecommendColumns(frameCount int) int {
	_, cols := RecommendGrid(frameCount)
	return cols
}

func RecommendRows(frameCount int) int {
	rows, _ := RecommendGrid(frameCount)
	return rows
}

func DefaultPlanner(frameCount int) *Planner {
	rows, columns := RecommendGrid(frameCount)
	margin := DefaultMargin
	gap := DefaultGap
	maxSheetWidth := 2*margin + columns*DefaultCellWidth
	if columns > 1 {
		maxSheetWidth += (columns - 1) * gap
	}
	maxSheetHeight := 2*margin + rows*DefaultCellHeight
	if rows > 1 {
		maxSheetHeight += (rows - 1) * gap
	}
	return NewPlanner(
		frameCount,
		DefaultCellWidth,
		DefaultCellHeight,
		maxSheetWidth,
		maxSheetHeight,
		margin,
		margin,
		gap,
		gap,
		columns,
		rows,
	)
}
