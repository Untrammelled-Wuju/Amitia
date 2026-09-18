package generationlayout

const (
	DefaultCellWidth  = 1024
	DefaultCellHeight = 1024
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
	{FrameCount: 12, Rows: 3, Columns: 4, Name: "4x3"},
}

func Presets() []GridPreset {
	return gridPresets
}

func RecommendGrid(frameCount int) (rows, columns int) {
	for _, p := range gridPresets {
		if p.FrameCount == frameCount {
			return p.Rows, p.Columns
		}
	}
	return 3, 4
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
