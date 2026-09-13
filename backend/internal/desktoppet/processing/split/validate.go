package split

import (
	"fmt"
	"image"

	"github.com/u-ai/backend/internal/desktoppet/processing/source"
)

const (
	figureMinComponentRatio = 0.25
	figureEdgeToleranceNum  = 15
	figureEdgeToleranceDen  = 100
	figureMinTolerance      = 12
)

func sheetHasContent(sheet *image.NRGBA, x, y int) bool {
	i := sheet.PixOffset(x, y)
	px := sheet.Pix[i : i+4]
	return px[3] >= 8 && !(px[0] >= 232 && px[1] >= 232 && px[2] >= 232)
}

func validateFiguresWithinCells(sheet *image.NRGBA, cells []source.SpriteSheetCell, expectedFrames int) error {
	b := sheet.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || len(cells) == 0 {
		return nil
	}

	labels := make([]int32, w*h)
	bboxes := make([][4]int, 0, 16)
	areas := make([]int32, 0, 16)
	stack := make([]int, 0, 8192)

	for y0 := 0; y0 < h; y0++ {
		for x0 := 0; x0 < w; x0++ {
			idx := y0*w + x0
			if labels[idx] != 0 || !sheetHasContent(sheet, b.Min.X+x0, b.Min.Y+y0) {
				continue
			}
			label := int32(len(areas) + 1)
			var minX, maxX, minY, maxY = x0, x0, y0, y0
			var area int32
			labels[idx] = label
			stack = append(stack[:0], idx)
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				area++
				cx, cy := cur%w, cur/w
				if cx < minX {
					minX = cx
				}
				if cx > maxX {
					maxX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cy > maxY {
					maxY = cy
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := cx+dx, cy+dy
						if nx < 0 || ny < 0 || nx >= w || ny >= h {
							continue
						}
						nIdx := ny*w + nx
						if labels[nIdx] == 0 && sheetHasContent(sheet, b.Min.X+nx, b.Min.Y+ny) {
							labels[nIdx] = label
							stack = append(stack, nIdx)
						}
					}
				}
			}
			bboxes = append(bboxes, [4]int{int(minX), int(minY), int(maxX), int(maxY)})
			areas = append(areas, area)
		}
	}

	if len(areas) == 0 {
		return nil
	}
	largest := int32(0)
	for _, a := range areas {
		if a > largest {
			largest = a
		}
	}
	minArea := int32(float64(largest) * figureMinComponentRatio)

	for i, bbox := range bboxes {
		if areas[i] < minArea {
			continue
		}
		if err := checkFigureWithinCells(bbox, cells, expectedFrames); err != nil {
			return err
		}
	}
	return nil
}

func checkFigureWithinCells(bbox [4]int, cells []source.SpriteSheetCell, expectedFrames int) error {
	for _, cell := range cells {
		cx0, cy0 := cell.X, cell.Y
		cx1, cy1 := cell.X+cell.Width, cell.Y+cell.Height
		if cx1 <= cx0 || cy1 <= cy0 {
			continue
		}
		tolX := cell.Width * figureEdgeToleranceNum / figureEdgeToleranceDen
		tolY := cell.Height * figureEdgeToleranceNum / figureEdgeToleranceDen
		if tolX < figureMinTolerance {
			tolX = figureMinTolerance
		}
		if tolY < figureMinTolerance {
			tolY = figureMinTolerance
		}
		if bbox[0] >= cx0-tolX && bbox[1] >= cy0-tolY && bbox[2] <= cx1+tolX && bbox[3] <= cy1+tolY {
			return nil
		}
	}
	return fmt.Errorf("split: sheet is not a valid animation sprite sheet: figure content at (%d,%d)-(%d,%d) crosses cell boundaries; regenerate a full %d-frame loop sprite sheet with one complete character per cell",
		bbox[0], bbox[1], bbox[2], bbox[3], expectedFrames)
}
