package split

import (
	"image"
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/processing/source"
)

func fillFigure(img *image.NRGBA, x0, y0, x1, y1 int) {
	c := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func testLayout() *source.SpriteSheetLayoutSnapshot {
	return &source.SpriteSheetLayoutSnapshot{
		Rows:         3,
		Columns:      4,
		CellWidth:    362,
		CellHeight:   362,
		ReadingOrder: "row_major",
	}
}

func TestValidateFiguresRejectsCrossCellContent(t *testing.T) {
	sheet := image.NewNRGBA(image.Rect(0, 0, 1448, 1086))
	fillFigure(sheet, 100, 50, 200, 1000)

	if err := validateFiguresWithinCells(sheet, source.GenerateCells(testLayout()), 12); err == nil {
		t.Fatalf("figure spanning multiple rows must be rejected")
	}
}

func TestValidateFiguresAcceptsPerCellFigures(t *testing.T) {
	sheet := image.NewNRGBA(image.Rect(0, 0, 1448, 1086))
	for _, cell := range source.GenerateCells(testLayout()) {
		if cell.Empty {
			continue
		}
		fillFigure(sheet, cell.X+140, cell.Y+80, cell.X+220, cell.Y+300)
	}

	if err := validateFiguresWithinCells(sheet, source.GenerateCells(testLayout()), 12); err != nil {
		t.Fatalf("per-cell figures should pass: %v", err)
	}
}
