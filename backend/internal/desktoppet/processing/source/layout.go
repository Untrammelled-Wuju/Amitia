package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func ValidateLayout(layout *SpriteSheetLayoutSnapshot, logicalFrameCount int) error {
	if layout == nil {
		return errors.New("layout is nil")
	}
	if layout.Rows <= 0 {
		return errors.New("rows must be positive")
	}
	if layout.Columns <= 0 {
		return errors.New("columns must be positive")
	}
	if layout.CellWidth <= 0 {
		return errors.New("cellWidth must be positive")
	}
	if layout.CellHeight <= 0 {
		return errors.New("cellHeight must be positive")
	}
	if layout.GapX < 0 || layout.GapY < 0 {
		return errors.New("gaps must be non-negative")
	}
	if layout.MarginTop < 0 || layout.MarginRight < 0 || layout.MarginBottom < 0 || layout.MarginLeft < 0 {
		return errors.New("margins must be non-negative")
	}
	if layout.Rows > 16 {
		return errors.New("rows must not exceed 16")
	}
	if layout.Columns > 16 {
		return errors.New("columns must not exceed 16")
	}
	totalCells := layout.Rows * layout.Columns
	if totalCells > 256 {
		return errors.New("total cells must not exceed 256")
	}
	if totalCells < logicalFrameCount {
		return fmt.Errorf("rows*columns(%d) < logicalFrameCount(%d)", totalCells, logicalFrameCount)
	}
	if !strings.HasPrefix(layout.ReadingOrder, "row_major") {
		return fmt.Errorf("readingOrder must be row_major, got: %s", layout.ReadingOrder)
	}

	cells := GenerateCells(layout)
	if len(cells) != totalCells {
		return fmt.Errorf("cell count(%d) != rows*columns(%d)", len(cells), totalCells)
	}

	for _, idx := range layout.EmptyCellIndexes {
		if idx < 0 || idx >= totalCells {
			return fmt.Errorf("emptyCellIndex %d out of range [0, %d)", idx, totalCells)
		}
	}

	expectedW, expectedH := ComputeExpectedSize(layout)

	frameSet := make(map[int]bool)
	frameIndexes := make([]int, 0, logicalFrameCount)

	for i, cell := range cells {
		if cell.X < 0 || cell.Y < 0 {
			return fmt.Errorf("cell %d: position must be non-negative", i)
		}
		if cell.X+cell.Width > expectedW {
			return fmt.Errorf("cell %d: right edge %d exceeds expected width %d", i, cell.X+cell.Width, expectedW)
		}
		if cell.Y+cell.Height > expectedH {
			return fmt.Errorf("cell %d: bottom edge %d exceeds expected height %d", i, cell.Y+cell.Height, expectedH)
		}

		if cell.Empty {
			if cell.FrameIndex != nil {
				return fmt.Errorf("cell %d: empty cell must not have frame index", i)
			}
			continue
		}

		if cell.FrameIndex == nil {
			return fmt.Errorf("cell %d: non-empty cell must have frame index", i)
		}

		fi := *cell.FrameIndex
		if fi < 0 {
			return fmt.Errorf("cell %d: frame index %d must be non-negative", i, fi)
		}
		if frameSet[fi] {
			return fmt.Errorf("cell %d: duplicate frame index %d", i, fi)
		}
		frameSet[fi] = true
		frameIndexes = append(frameIndexes, fi)
	}

	sort.Ints(frameIndexes)
	for i, fi := range frameIndexes {
		if fi != i {
			return fmt.Errorf("frame indexes must be contiguous 0..N-1, gap at position %d (value %d)", i, fi)
		}
	}

	if len(frameIndexes) != logicalFrameCount {
		return fmt.Errorf("frame count(%d) != logicalFrameCount(%d)", len(frameIndexes), logicalFrameCount)
	}

	return nil
}

func ComputeExpectedSize(layout *SpriteSheetLayoutSnapshot) (int, int) {
	if layout == nil {
		return 0, 0
	}
	width := layout.MarginLeft + layout.Columns*layout.CellWidth + (layout.Columns-1)*layout.GapX + layout.MarginRight
	height := layout.MarginTop + layout.Rows*layout.CellHeight + (layout.Rows-1)*layout.GapY + layout.MarginBottom
	return width, height
}

func GenerateCells(layout *SpriteSheetLayoutSnapshot) []SpriteSheetCell {
	if layout == nil {
		return nil
	}
	if len(layout.Cells) > 0 {
		return layout.Cells
	}

	totalCells := layout.Rows * layout.Columns
	emptySet := make(map[int]bool)
	for _, idx := range layout.EmptyCellIndexes {
		emptySet[idx] = true
	}

	cells := make([]SpriteSheetCell, 0, totalCells)
	frameIdx := 0
	for row := 0; row < layout.Rows; row++ {
		for col := 0; col < layout.Columns; col++ {
			cellIndex := row*layout.Columns + col
			cell := SpriteSheetCell{
				CellIndex: cellIndex,
				Row:       row,
				Column:    col,
				X:         layout.MarginLeft + col*(layout.CellWidth+layout.GapX),
				Y:         layout.MarginTop + row*(layout.CellHeight+layout.GapY),
				Width:     layout.CellWidth,
				Height:    layout.CellHeight,
				Empty:     emptySet[cellIndex],
			}
			if !cell.Empty {
				fi := frameIdx
				cell.FrameIndex = &fi
				frameIdx++
			}
			cells = append(cells, cell)
		}
	}

	return cells
}

func ComputeLayoutHash(layout *SpriteSheetLayoutSnapshot) string {
	if layout == nil {
		return ""
	}
	data, err := json.Marshal(layout)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
