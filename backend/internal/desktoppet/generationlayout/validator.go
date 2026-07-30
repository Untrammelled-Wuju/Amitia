package generationlayout

import "fmt"

const (
	ErrCodeLayoutNil             = "LAYOUT_NIL"
	ErrCodeCapacityInsufficient  = "CAPACITY_INSUFFICIENT"
	ErrCodeDuplicateFrameMapping = "DUPLICATE_FRAME_MAPPING"
	ErrCodeFrameMissing          = "FRAME_MISSING"
	ErrCodeSheetSizeMismatch     = "SHEET_SIZE_MISMATCH"
	ErrCodeSheetSizeExceedsLimit = "SHEET_SIZE_EXCEEDS_LIMIT"
	ErrCodeEmptyCellNotMarked    = "EMPTY_CELL_NOT_MARKED"
	ErrCodeFrameCountMismatch    = "FRAME_COUNT_MISMATCH"
	ErrCodeInvalidFrameCount     = "INVALID_FRAME_COUNT"
	ErrCodeInvalidCellSize       = "INVALID_CELL_SIZE"
	ErrCodeInvalidSheetLimit     = "INVALID_SHEET_LIMIT"
	ErrCodeZeroCapacity          = "ZERO_CAPACITY"
)

type LayoutError struct {
	Code    string
	Message string
}

func (e *LayoutError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func Validate(result *LayoutResult) error {
	if result == nil {
		return &LayoutError{Code: ErrCodeLayoutNil, Message: "layout result is nil"}
	}

	frameSet := make(map[int]bool)
	totalUsed := 0

	for segIdx, seg := range result.Segments {
		layout := seg.SheetLayout
		capacity := layout.Rows * layout.Columns

		if capacity < layout.FrameCount {
			return &LayoutError{
				Code:    ErrCodeCapacityInsufficient,
				Message: fmt.Sprintf("segment %d: rows*columns(%d) < frameCount(%d)", segIdx, capacity, layout.FrameCount),
			}
		}

		if layout.TotalCells != len(layout.Cells) {
			return &LayoutError{
				Code:    ErrCodeSheetSizeMismatch,
				Message: fmt.Sprintf("segment %d: TotalCells(%d) != len(Cells)(%d)", segIdx, layout.TotalCells, len(layout.Cells)),
			}
		}

		emptyCount := 0
		for _, idx := range layout.EmptyCells {
			if idx < 0 || idx >= len(layout.Cells) {
				return &LayoutError{
					Code:    ErrCodeEmptyCellNotMarked,
					Message: fmt.Sprintf("segment %d: EmptyCells index %d out of range [0,%d)", segIdx, idx, len(layout.Cells)),
				}
			}
			if !layout.Cells[idx].IsEmpty {
				return &LayoutError{
					Code:    ErrCodeEmptyCellNotMarked,
					Message: fmt.Sprintf("segment %d: EmptyCells index %d is not an empty cell", segIdx, idx),
				}
			}
		}

		for _, cell := range layout.Cells {
			if cell.IsEmpty {
				if cell.FrameIndex != -1 {
					return &LayoutError{
						Code:    ErrCodeEmptyCellNotMarked,
						Message: fmt.Sprintf("segment %d: empty cell at row=%d col=%d has FrameIndex=%d", segIdx, cell.Row, cell.Column, cell.FrameIndex),
					}
				}
				emptyCount++
			} else {
				if cell.FrameIndex < 0 {
					return &LayoutError{
						Code:    ErrCodeFrameMissing,
						Message: fmt.Sprintf("segment %d: non-empty cell at row=%d col=%d has invalid FrameIndex=%d", segIdx, cell.Row, cell.Column, cell.FrameIndex),
					}
				}
				if frameSet[cell.FrameIndex] {
					return &LayoutError{
						Code:    ErrCodeDuplicateFrameMapping,
						Message: fmt.Sprintf("segment %d: frame %d mapped to multiple cells", segIdx, cell.FrameIndex),
					}
				}
				frameSet[cell.FrameIndex] = true
				totalUsed++
			}
		}

		if len(layout.EmptyCells) != emptyCount {
			return &LayoutError{
				Code:    ErrCodeEmptyCellNotMarked,
				Message: fmt.Sprintf("segment %d: EmptyCells count(%d) != actual empty cells(%d)", segIdx, len(layout.EmptyCells), emptyCount),
			}
		}

		if layout.UsedCells != layout.FrameCount {
			return &LayoutError{
				Code:    ErrCodeFrameCountMismatch,
				Message: fmt.Sprintf("segment %d: UsedCells(%d) != FrameCount(%d)", segIdx, layout.UsedCells, layout.FrameCount),
			}
		}

		expectedWidth := 2*layout.MarginX + layout.Columns*layout.CellWidth + (layout.Columns-1)*layout.GapX
		if layout.SheetWidth != expectedWidth {
			return &LayoutError{
				Code:    ErrCodeSheetSizeMismatch,
				Message: fmt.Sprintf("segment %d: SheetWidth(%d) != expected(%d)", segIdx, layout.SheetWidth, expectedWidth),
			}
		}
		expectedHeight := 2*layout.MarginY + layout.Rows*layout.CellHeight + (layout.Rows-1)*layout.GapY
		if layout.SheetHeight != expectedHeight {
			return &LayoutError{
				Code:    ErrCodeSheetSizeMismatch,
				Message: fmt.Sprintf("segment %d: SheetHeight(%d) != expected(%d)", segIdx, layout.SheetHeight, expectedHeight),
			}
		}
	}

	if len(frameSet) != result.FrameCount {
		return &LayoutError{
			Code:    ErrCodeFrameCountMismatch,
			Message: fmt.Sprintf("unique frames(%d) != FrameCount(%d)", len(frameSet), result.FrameCount),
		}
	}

	if result.TotalUsedCells != totalUsed {
		return &LayoutError{
			Code:    ErrCodeFrameCountMismatch,
			Message: fmt.Sprintf("TotalUsedCells(%d) != actual used(%d)", result.TotalUsedCells, totalUsed),
		}
	}

	return nil
}

func ValidateAgainstLimits(result *LayoutResult, maxSheetWidth, maxSheetHeight int) error {
	if err := Validate(result); err != nil {
		return err
	}

	for segIdx, seg := range result.Segments {
		layout := seg.SheetLayout
		if layout.SheetWidth > maxSheetWidth {
			return &LayoutError{
				Code:    ErrCodeSheetSizeExceedsLimit,
				Message: fmt.Sprintf("segment %d: SheetWidth(%d) > maxSheetWidth(%d)", segIdx, layout.SheetWidth, maxSheetWidth),
			}
		}
		if layout.SheetHeight > maxSheetHeight {
			return &LayoutError{
				Code:    ErrCodeSheetSizeExceedsLimit,
				Message: fmt.Sprintf("segment %d: SheetHeight(%d) > maxSheetHeight(%d)", segIdx, layout.SheetHeight, maxSheetHeight),
			}
		}
	}

	return nil
}
