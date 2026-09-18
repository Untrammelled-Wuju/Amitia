package split

import (
	"image"

	"github.com/u-ai/backend/internal/desktoppet/processing/source"
)

type gridProfile struct {
	emptyRows    []bool
	emptyCols    []bool
	sheetW       int
	sheetH       int
	marginTop    int
	marginBottom int
	marginLeft   int
	marginRight  int
}

func buildGridProfile(sheet *image.NRGBA, layout *source.SpriteSheetLayoutSnapshot) *gridProfile {
	b := sheet.Bounds()
	w, h := b.Dx(), b.Dy()
	p := &gridProfile{
		emptyRows:    make([]bool, h),
		emptyCols:    make([]bool, w),
		sheetW:       w,
		sheetH:       h,
		marginTop:    layout.MarginTop,
		marginBottom: layout.MarginBottom,
		marginLeft:   layout.MarginLeft,
		marginRight:  layout.MarginRight,
	}

	rowNonBg := make([]int, h)
	colNonBg := make([]int, w)
	for y := 0; y < h; y++ {
		rowStart := y * sheet.Stride
		for x := 0; x < w; x++ {
			i := rowStart + x*4
			if isContentPixel(sheet.Pix[i : i+4]) {
				rowNonBg[y]++
				colNonBg[x]++
			}
		}
	}

	rowTol := w / 64
	if rowTol < 8 {
		rowTol = 8
	}
	colTol := h / 64
	if colTol < 8 {
		colTol = 8
	}
	for y := 0; y < h; y++ {
		p.emptyRows[y] = rowNonBg[y] <= rowTol
	}
	for x := 0; x < w; x++ {
		p.emptyCols[x] = colNonBg[x] <= colTol
	}
	return p
}

func isContentPixel(px []uint8) bool {
	if px[3] < 8 {
		return false
	}
	return !(px[0] >= 232 && px[1] >= 232 && px[2] >= 232)
}

func (p *gridProfile) findBoundary(planned, tol int, empty []bool, limit int) int {
	if planned < 0 {
		planned = 0
	}
	if planned > limit {
		planned = limit
	}
	lo := planned - tol
	if lo < 0 {
		lo = 0
	}
	hi := planned + tol
	if hi > limit {
		hi = limit
	}
	bestMid := -1
	bestDist := tol + 1
	runStart := -1
	for i := lo; i <= hi+1; i++ {
		emptyHere := i <= hi && i >= 0 && i < len(empty) && empty[i]
		if emptyHere && runStart < 0 {
			runStart = i
		}
		if (!emptyHere || i == hi+1) && runStart >= 0 {
			runEnd := i - 1
			if i == hi+1 && emptyHere {
				runEnd = hi
			}
			if runEnd-runStart+1 >= 3 {
				mid := (runStart + runEnd) / 2
				dist := mid - planned
				if dist < 0 {
					dist = -dist
				}
				if dist < bestDist {
					bestDist = dist
					bestMid = mid
				}
			}
			runStart = -1
		}
	}
	if bestMid >= 0 {
		return bestMid
	}
	return planned
}

func refineCellRects(sheet *image.NRGBA, layout *source.SpriteSheetLayoutSnapshot) [][4]int {
	rows, cols := layout.Rows, layout.Columns
	b := sheet.Bounds()
	p := buildGridProfile(sheet, layout)

	tolY := layout.CellHeight / 6
	if tolY < 16 {
		tolY = 16
	}
	tolX := layout.CellWidth / 6
	if tolX < 16 {
		tolX = 16
	}

	rowBounds := make([]int, rows+1)
	for r := 0; r <= rows; r++ {
		if r == 0 {
			planned := layout.MarginTop
			if planned > b.Dy() {
				planned = b.Dy()
			}
			rowBounds[0] = p.findBoundary(planned, tolY, p.emptyRows, b.Dy()-1)
			continue
		}
		if r == rows {
			planned := b.Dy() - layout.MarginBottom
			if planned < 0 {
				planned = 0
			}
			rowBounds[rows] = p.findBoundary(planned, tolY, p.emptyRows, b.Dy()-1)
			continue
		}
		planned := layout.MarginTop + r*(layout.CellHeight+layout.GapY)
		if planned > b.Dy() {
			planned = b.Dy()
		}
		rowBounds[r] = p.findBoundary(planned, tolY, p.emptyRows, b.Dy()-1)
	}

	colBounds := make([]int, cols+1)
	for c := 0; c <= cols; c++ {
		if c == 0 {
			planned := layout.MarginLeft
			if planned > b.Dx() {
				planned = b.Dx()
			}
			colBounds[0] = p.findBoundary(planned, tolX, p.emptyCols, b.Dx()-1)
			continue
		}
		if c == cols {
			planned := b.Dx() - layout.MarginRight
			if planned < 0 {
				planned = 0
			}
			colBounds[cols] = p.findBoundary(planned, tolX, p.emptyCols, b.Dx()-1)
			continue
		}
		planned := layout.MarginLeft + c*(layout.CellWidth+layout.GapX)
		if planned > b.Dx() {
			planned = b.Dx()
		}
		colBounds[c] = p.findBoundary(planned, tolX, p.emptyCols, b.Dx()-1)
	}

	for r := 0; r < rows; r++ {
		if rowBounds[r+1]-rowBounds[r] < layout.CellHeight/2 {
			rowBounds[r] = clampPlannedRow(layout, r, b.Dy())
			rowBounds[r+1] = clampPlannedRow(layout, r+1, b.Dy())
		}
	}
	for c := 0; c < cols; c++ {
		if colBounds[c+1]-colBounds[c] < layout.CellWidth/2 {
			colBounds[c] = clampPlannedCol(layout, c, b.Dx())
			colBounds[c+1] = clampPlannedCol(layout, c+1, b.Dx())
		}
	}

	rects := make([][4]int, rows*cols)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			rects[idx] = [4]int{colBounds[c], rowBounds[r], colBounds[c+1] - colBounds[c], rowBounds[r+1] - rowBounds[r]}
		}
	}
	return rects
}

func clampPlannedRow(layout *source.SpriteSheetLayoutSnapshot, edge, sheetH int) int {
	v := layout.MarginTop + edge*(layout.CellHeight+layout.GapY)
	if edge == layout.Rows {
		v = sheetH - layout.MarginBottom
	}
	if v < 0 {
		return 0
	}
	if v > sheetH {
		return sheetH
	}
	return v
}

func clampPlannedCol(layout *source.SpriteSheetLayoutSnapshot, edge, sheetW int) int {
	v := layout.MarginLeft + edge*(layout.CellWidth+layout.GapX)
	if edge == layout.Columns {
		v = sheetW - layout.MarginRight
	}
	if v < 0 {
		return 0
	}
	if v > sheetW {
		return sheetW
	}
	return v
}
