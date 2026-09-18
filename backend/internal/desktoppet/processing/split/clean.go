package split

import "image"

const bleedComponentRatio = 0.05

func cellHasContent(cell *image.NRGBA, x, y int) bool {
	b := cell.Bounds()
	i := cell.PixOffset(b.Min.X+x, b.Min.Y+y)
	px := cell.Pix[i : i+4]
	return px[3] >= 8 && !(px[0] >= 232 && px[1] >= 232 && px[2] >= 232)
}

func clearRow(cell *image.NRGBA, y int) {
	b := cell.Bounds()
	for x := 0; x < b.Dx(); x++ {
		i := cell.PixOffset(b.Min.X+x, b.Min.Y+y)
		px := cell.Pix[i : i+4]
		px[0], px[1], px[2], px[3] = 255, 255, 255, 255
	}
}

func trimTopBleedBand(cell *image.NRGBA) {
	b := cell.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return
	}
	limit := h / 4
	if limit < 8 {
		limit = 8
	}
	if limit > h {
		limit = h
	}
	wideThreshold := w / 4
	bandRows := 0
	for y := 0; y < limit; y++ {
		count := 0
		for x := 0; x < w; x++ {
			if cellHasContent(cell, x, y) {
				count++
			}
		}
		if count > wideThreshold {
			bandRows = y + 1
		} else {
			break
		}
	}
	if bandRows > 0 && bandRows < limit {
		for y := 0; y < bandRows; y++ {
			clearRow(cell, y)
		}
	}
}

func cleanCellContent(cell *image.NRGBA) {
	b := cell.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return
	}
	labels := make([]int32, w*h)
	sizes := make([]int32, 0, 16)
	stack := make([]int, 0, 4096)
	next := int32(0)

	at := func(x, y int) int { return y*w + x }
	isContent := func(x, y int) bool {
		i := cell.PixOffset(b.Min.X+x, b.Min.Y+y)
		px := cell.Pix[i : i+4]
		return px[3] >= 8 && !(px[0] >= 232 && px[1] >= 232 && px[2] >= 232)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := at(x, y)
			if labels[idx] != 0 || !isContent(x, y) {
				continue
			}
			next++
			var size int32
			labels[idx] = next
			stack = append(stack[:0], idx)
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				size++
				cx := cur % w
				cy := cur / w
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := cx+dx, cy+dy
						if nx < 0 || ny < 0 || nx >= w || ny >= h {
							continue
						}
						nIdx := at(nx, ny)
						if labels[nIdx] == 0 && isContent(nx, ny) {
							labels[nIdx] = next
							stack = append(stack, nIdx)
						}
					}
				}
			}
			sizes = append(sizes, size)
		}
	}

	if len(sizes) <= 1 {
		return
	}
	largest := int32(0)
	for _, s := range sizes {
		if s > largest {
			largest = s
		}
	}
	minKeep := int32(float64(largest) * bleedComponentRatio)
	for id := int32(1); id <= int32(len(sizes)); id++ {
		if sizes[id-1] >= minKeep {
			continue
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				idx := at(x, y)
				if labels[idx] == id {
					i := cell.PixOffset(b.Min.X+x, b.Min.Y+y)
					px := cell.Pix[i : i+4]
					px[0], px[1], px[2], px[3] = 255, 255, 255, 255
				}
			}
		}
	}
}
