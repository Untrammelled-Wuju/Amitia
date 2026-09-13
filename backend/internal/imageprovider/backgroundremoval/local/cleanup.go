package local

import (
	"image"
)

const (
	nearWhiteMinChannel   = 230
	nearWhiteMaxSpread    = 18
	edgeBandCoverageNum   = 3
	edgeBandCoverageDen   = 5
	rescueComponentRatio  = 0.005
	rescueComponentMinPix = 64
	rescueMaxAreaRatio    = 0.04
	keyFloodAlpha         = 128

	floodBrightMinChannel = 226
	floodBrightMaxSpread  = 10

	hazeWhiteMinChannel = 238
	hazeWhiteMaxSpread  = 14
	hazeMaxAlpha        = 230
	hazeMaxAreaRatio    = 0.003
)

func clearDetachedWhiteHaze(img *image.NRGBA, bounds image.Rectangle) {
	w := bounds.Dx()
	h := bounds.Dy()
	if w == 0 || h == 0 || img == nil {
		return
	}
	alphaAt := func(idx int) int {
		return int(img.Pix[img.PixOffset(bounds.Min.X+idx%w, bounds.Min.Y+idx/w)+3])
	}
	labels := make([]int, w*h)
	comps := make([][]int, 0, 64)
	stack := make([]int, 0, 1024)
	for start := 0; start < w*h; start++ {
		if labels[start] != 0 || alphaAt(start) < keyFloodAlpha {
			continue
		}
		id := len(comps) + 1
		comp := make([]int, 0, 256)
		labels[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, cur)
			cx, cy := cur%w, cur/w
			tryVisit := func(n int) {
				if n < 0 || n >= w*h || labels[n] != 0 || alphaAt(n) < keyFloodAlpha {
					return
				}
				labels[n] = id
				stack = append(stack, n)
			}
			if cx > 0 {
				tryVisit(cur - 1)
			}
			if cx < w-1 {
				tryVisit(cur + 1)
			}
			if cy > 0 {
				tryVisit(cur - w)
			}
			if cy < h-1 {
				tryVisit(cur + w)
			}
		}
		comps = append(comps, comp)
	}
	if len(comps) <= 1 {
		return
	}
	largest := 0
	for i, c := range comps {
		if len(c) > len(comps[largest]) {
			largest = i
		}
	}
	maxArea := int(float64(w*h) * hazeMaxAreaRatio)
	for i, comp := range comps {
		if i == largest || len(comp) > maxArea {
			continue
		}
		var alphaSum, rSum, gSum, bSum int64
		for _, idx := range comp {
			off := img.PixOffset(bounds.Min.X+idx%w, bounds.Min.Y+idx/w)
			a := int(img.Pix[off+3])
			alphaSum += int64(a)
			rSum += int64(img.Pix[off]) * int64(a)
			gSum += int64(img.Pix[off+1]) * int64(a)
			bSum += int64(img.Pix[off+2]) * int64(a)
		}
		if alphaSum == 0 {
			continue
		}
		avgAlpha := alphaSum / int64(len(comp))
		avgR := rSum / alphaSum
		avgG := gSum / alphaSum
		avgB := bSum / alphaSum
		mn := min(avgR, min(avgG, avgB))
		mx := max(avgR, max(avgG, avgB))
		if avgAlpha >= hazeMaxAlpha || mn < hazeWhiteMinChannel || int(mx)-int(mn) > hazeWhiteMaxSpread {
			continue
		}
		for _, idx := range comp {
			off := img.PixOffset(bounds.Min.X+idx%w, bounds.Min.Y+idx/w)
			img.Pix[off+3] = 0
		}
	}
}

func removeBorderNearWhiteRegions(mask [][]bool, img *image.NRGBA, bounds image.Rectangle) {
	h := len(mask)
	if h == 0 {
		return
	}
	w := len(mask[0])
	if w == 0 || img == nil {
		return
	}

	removable := make([][]bool, h)
	for y := 0; y < h; y++ {
		removable[y] = make([]bool, w)
		for x := 0; x < w; x++ {
			if !mask[y][x] {
				continue
			}
			idx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b := img.Pix[idx], img.Pix[idx+1], img.Pix[idx+2]
			mn := min(r, min(g, b))
			mx := max(r, max(g, b))
			removable[y][x] = mn >= nearWhiteMinChannel && int(mx)-int(mn) <= nearWhiteMaxSpread
		}
	}

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}
	stack := make([][2]int, 0, w+h)
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return
		}
		if visited[y][x] || !removable[y][x] {
			return
		}
		visited[y][x] = true
		stack = append(stack, [2]int{x, y})
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		cx, cy := cur[0], cur[1]
		push(cx-1, cy)
		push(cx+1, cy)
		push(cx, cy-1)
		push(cx, cy+1)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if visited[y][x] {
				mask[y][x] = false
			}
		}
	}
}

func trimMaskEdgeBands(mask [][]bool) {
	h := len(mask)
	if h == 0 {
		return
	}
	w := len(mask[0])
	if w == 0 {
		return
	}

	labels := make([]int32, w*h)
	sizes := make([]int32, 0, 8)
	stack := make([]int, 0, 1024)
	next := int32(0)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if labels[idx] != 0 || !mask[y][x] {
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
				cx, cy := cur%w, cur/w
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
						if labels[nIdx] == 0 && mask[ny][nx] {
							labels[nIdx] = next
							stack = append(stack, nIdx)
						}
					}
				}
			}
			sizes = append(sizes, size)
		}
	}
	if next <= 1 {
		return
	}
	largest := int32(0)
	largestLabel := int32(0)
	for i, s := range sizes {
		if s > largest {
			largest = s
			largestLabel = int32(i + 1)
		}
	}

	rowThreshold := w * edgeBandCoverageNum / edgeBandCoverageDen
	colThreshold := h * edgeBandCoverageNum / edgeBandCoverageDen

	clearNonLargestInRow := func(y int) {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if mask[y][x] && labels[idx] != largestLabel {
				mask[y][x] = false
			}
		}
	}
	clearNonLargestInCol := func(x int) {
		for y := 0; y < h; y++ {
			idx := y*w + x
			if mask[y][x] && labels[idx] != largestLabel {
				mask[y][x] = false
			}
		}
	}

	for y := 0; y < h; y++ {
		count := 0
		for x := 0; x < w; x++ {
			if mask[y][x] {
				count++
			}
		}
		if count < rowThreshold {
			break
		}
		clearNonLargestInRow(y)
	}
	for y := h - 1; y >= 0; y-- {
		count := 0
		for x := 0; x < w; x++ {
			if mask[y][x] {
				count++
			}
		}
		if count < rowThreshold {
			break
		}
		clearNonLargestInRow(y)
	}
	for x := 0; x < w; x++ {
		count := 0
		for y := 0; y < h; y++ {
			if mask[y][x] {
				count++
			}
		}
		if count < colThreshold {
			break
		}
		clearNonLargestInCol(x)
	}
	for x := w - 1; x >= 0; x-- {
		count := 0
		for y := 0; y < h; y++ {
			if mask[y][x] {
				count++
			}
		}
		if count < colThreshold {
			break
		}
		clearNonLargestInCol(x)
	}
}

func removeByColorConnectedNRGBA(img *image.NRGBA, bgR, bgG, bgB uint8, threshold int) *image.NRGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	result := image.NewNRGBA(bounds)

	thresholdSq := threshold * threshold
	featherStart := int(float64(threshold) * 0.7)
	featherStartSq := featherStart * featherStart

	keyed := make([]uint8, w*h)
	passable := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			r := int(img.Pix[idx])
			g := int(img.Pix[idx+1])
			b := int(img.Pix[idx+2])
			dr := r - int(bgR)
			dg := g - int(bgG)
			db := b - int(bgB)
			distSq := dr*dr + dg*dg + db*db
			var alpha uint8
			switch {
			case distSq <= featherStartSq:
				alpha = 0
			case distSq <= thresholdSq:
				if thresholdSq == featherStartSq {
					alpha = 255
				} else {
					ratio := float64(distSq-featherStartSq) / float64(thresholdSq-featherStartSq)
					alpha = uint8(255 * ratio)
				}
			default:
				alpha = 255
			}
			k := y*w + x
			keyed[k] = alpha
			mn := min(r, min(g, b))
			mx := max(r, max(g, b))
			passable[k] = mn >= floodBrightMinChannel && mx-mn <= floodBrightMaxSpread
		}
	}

	visited := make([]bool, w*h)
	stack := make([]int, 0, 4096)
	push := func(idx int) {
		if visited[idx] {
			return
		}
		if keyed[idx] >= keyFloodAlpha && !passable[idx] {
			return
		}
		visited[idx] = true
		stack = append(stack, idx)
	}
	for x := 0; x < w; x++ {
		push(x)
		push((h-1)*w + x)
	}
	for y := 0; y < h; y++ {
		push(y * w)
		push(y*w + w - 1)
	}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		cx, cy := cur%w, cur/w
		if cx > 0 {
			push(cur - 1)
		}
		if cx < w-1 {
			push(cur + 1)
		}
		if cy > 0 {
			push(cur - w)
		}
		if cy < h-1 {
			push(cur + w)
		}
	}

	rescued := make([]bool, w*h)
	seen := make([]bool, w*h)
	rescueMinArea := int(float64(w*h) * rescueComponentRatio)
	if rescueMinArea < rescueComponentMinPix {
		rescueMinArea = rescueComponentMinPix
	}
	rescueMaxArea := int(float64(w*h) * rescueMaxAreaRatio)
	for start := 0; start < w*h; start++ {
		if visited[start] || seen[start] || keyed[start] >= keyFloodAlpha {
			continue
		}
		component := make([]int, 0, 256)
		seen[start] = true
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, cur)
			cx, cy := cur%w, cur/w
			tryVisit := func(nIdx int) {
				if nIdx < 0 || nIdx >= w*h || seen[nIdx] || visited[nIdx] || keyed[nIdx] >= keyFloodAlpha {
					return
				}
				seen[nIdx] = true
				stack = append(stack, nIdx)
			}
			if cx > 0 {
				tryVisit(cur - 1)
			}
			if cx < w-1 {
				tryVisit(cur + 1)
			}
			if cy > 0 {
				tryVisit(cur - w)
			}
			if cy < h-1 {
				tryVisit(cur + w)
			}
		}
		if len(component) >= rescueMinArea && len(component) <= rescueMaxArea {
			for _, idx := range component {
				rescued[idx] = true
			}
		}
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			dstIdx := result.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			result.Pix[dstIdx] = img.Pix[srcIdx]
			result.Pix[dstIdx+1] = img.Pix[srcIdx+1]
			result.Pix[dstIdx+2] = img.Pix[srcIdx+2]
			k := y*w + x
			switch {
			case visited[k]:
				result.Pix[dstIdx+3] = 0
			case rescued[k]:
				result.Pix[dstIdx+3] = 255
			default:
				result.Pix[dstIdx+3] = keyed[k]
			}
		}
	}

	return result
}
