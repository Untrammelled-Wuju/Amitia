package generationlayout

type Planner struct {
	FrameCount     int
	CellWidth      int
	CellHeight     int
	MaxSheetWidth  int
	MaxSheetHeight int
	MarginX        int
	MarginY        int
	GapX           int
	GapY           int
	MaxColumns     int
	MaxRows        int
	ReadingOrder   ReadingOrder
}

func NewPlanner(frameCount, cellWidth, cellHeight, maxSheetWidth, maxSheetHeight, marginX, marginY, gapX, gapY, maxColumns, maxRows int) *Planner {
	return &Planner{
		FrameCount:     frameCount,
		CellWidth:      cellWidth,
		CellHeight:     cellHeight,
		MaxSheetWidth:  maxSheetWidth,
		MaxSheetHeight: maxSheetHeight,
		MarginX:        marginX,
		MarginY:        marginY,
		GapX:           gapX,
		GapY:           gapY,
		MaxColumns:     maxColumns,
		MaxRows:        maxRows,
		ReadingOrder:   ReadingRowMajorLeftToRight,
	}
}

func (p *Planner) WithReadingOrder(order ReadingOrder) *Planner {
	p.ReadingOrder = order
	return p
}

func (p *Planner) computeColumns() int {
	usableWidth := p.MaxSheetWidth - 2*p.MarginX
	cols := 1
	if usableWidth >= p.CellWidth {
		cols = (usableWidth + p.GapX) / (p.CellWidth + p.GapX)
	}
	if cols < 1 {
		cols = 1
	}
	if p.MaxColumns > 0 && cols > p.MaxColumns {
		cols = p.MaxColumns
	}
	return cols
}

func (p *Planner) computeRows() int {
	usableHeight := p.MaxSheetHeight - 2*p.MarginY
	rows := 1
	if usableHeight >= p.CellHeight {
		rows = (usableHeight + p.GapY) / (p.CellHeight + p.GapY)
	}
	if rows < 1 {
		rows = 1
	}
	if p.MaxRows > 0 && rows > p.MaxRows {
		rows = p.MaxRows
	}
	return rows
}

func (p *Planner) sheetWidth(columns int) int {
	return 2*p.MarginX + columns*p.CellWidth + (columns-1)*p.GapX
}

func (p *Planner) sheetHeight(rows int) int {
	return 2*p.MarginY + rows*p.CellHeight + (rows-1)*p.GapY
}

func (p *Planner) Plan() (*LayoutResult, error) {
	if p.FrameCount < 0 {
		return nil, &LayoutError{Code: ErrCodeInvalidFrameCount, Message: "frame count must be non-negative"}
	}
	if p.CellWidth <= 0 || p.CellHeight <= 0 {
		return nil, &LayoutError{Code: ErrCodeInvalidCellSize, Message: "cell width and height must be positive"}
	}
	if p.MaxSheetWidth <= 0 || p.MaxSheetHeight <= 0 {
		return nil, &LayoutError{Code: ErrCodeInvalidSheetLimit, Message: "max sheet dimensions must be positive"}
	}

	columns := p.computeColumns()
	rows := p.computeRows()

	capacity := rows * columns
	if capacity < 1 {
		return nil, &LayoutError{Code: ErrCodeZeroCapacity, Message: "sheet capacity is zero"}
	}

	sheetW := p.sheetWidth(columns)
	sheetH := p.sheetHeight(rows)

	if p.FrameCount == 0 {
		return &LayoutResult{
			Segments:       []SegmentLayout{},
			TotalSegments:  0,
			TotalSheets:    0,
			TotalCells:     0,
			TotalUsedCells: 0,
			FrameCount:     0,
			CellWidth:      p.CellWidth,
			CellHeight:     p.CellHeight,
			SheetWidth:     sheetW,
			SheetHeight:    sheetH,
		}, nil
	}

	segmentCount := (p.FrameCount + capacity - 1) / capacity
	segments := make([]SegmentLayout, 0, segmentCount)
	totalCells := 0
	totalUsed := 0

	frameStart := 0
	for i := 0; i < segmentCount; i++ {
		remaining := p.FrameCount - frameStart
		segFrames := capacity
		if segFrames > remaining {
			segFrames = remaining
		}

		cells := buildCells(p.ReadingOrder, rows, columns, frameStart, segFrames)
		emptyCells := make([]int, 0)
		for idx, cell := range cells {
			if cell.IsEmpty {
				emptyCells = append(emptyCells, idx)
			}
		}

		seg := SegmentLayout{
			SegmentIndex:    i,
			FrameStartIndex: frameStart,
			FrameEndIndex:   frameStart + segFrames - 1,
			FrameCount:      segFrames,
			SheetLayout: SheetLayout{
				Rows:         rows,
				Columns:      columns,
				CellWidth:    p.CellWidth,
				CellHeight:   p.CellHeight,
				MarginX:      p.MarginX,
				MarginY:      p.MarginY,
				GapX:         p.GapX,
				GapY:         p.GapY,
				SheetWidth:   sheetW,
				SheetHeight:  sheetH,
				ReadingOrder: p.ReadingOrder,
				Cells:        cells,
				EmptyCells:   emptyCells,
				TotalCells:   len(cells),
				UsedCells:    segFrames,
				FrameCount:   segFrames,
			},
		}
		segments = append(segments, seg)
		totalCells += len(cells)
		totalUsed += segFrames
		frameStart += segFrames
	}

	return &LayoutResult{
		Segments:       segments,
		TotalSegments:  segmentCount,
		TotalSheets:    segmentCount,
		TotalCells:     totalCells,
		TotalUsedCells: totalUsed,
		FrameCount:     p.FrameCount,
		CellWidth:      p.CellWidth,
		CellHeight:     p.CellHeight,
		SheetWidth:     sheetW,
		SheetHeight:    sheetH,
	}, nil
}

func buildCells(order ReadingOrder, rows, columns, frameStart, frameCount int) []Cell {
	total := rows * columns
	cells := make([]Cell, total)

	for i := 0; i < total; i++ {
		var row, col int
		switch order {
		case ReadingRowMajorLeftToRight:
			row = i / columns
			col = i % columns
		case ReadingRowMajorRightToLeft:
			row = i / columns
			col = columns - 1 - (i % columns)
		case ReadingColumnMajor:
			row = i % rows
			col = i / rows
		default:
			row = i / columns
			col = i % columns
		}

		cell := Cell{
			Row:    row,
			Column: col,
		}

		if i < frameCount {
			cell.FrameIndex = frameStart + i
			cell.IsEmpty = false
		} else {
			cell.FrameIndex = -1
			cell.IsEmpty = true
		}
		cells[i] = cell
	}

	return cells
}

func (p *Planner) RecommendColumns() int {
	return p.computeColumns()
}

func (p *Planner) RecommendRows() int {
	return p.computeRows()
}

func (p *Planner) Capacity() int {
	return p.computeRows() * p.computeColumns()
}
