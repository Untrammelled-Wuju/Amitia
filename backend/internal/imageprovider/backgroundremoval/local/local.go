// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package local

import (
	"context"
	"image"
	"image/color"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

const (
	defaultMinSubjectAreaRatio = 0.005
	defaultAlphaThreshold      = 10
	defaultEdgeErodeRadius     = 1
	defaultColorThreshold      = 30
)

type LocalProvider struct {
	minSubjectAreaRatio float64
	alphaThreshold      uint8
	edgeErodeRadius     int
	colorThreshold      int
}

func NewLocalProvider() *LocalProvider {
	return &LocalProvider{
		minSubjectAreaRatio: defaultMinSubjectAreaRatio,
		alphaThreshold:      defaultAlphaThreshold,
		edgeErodeRadius:     defaultEdgeErodeRadius,
		colorThreshold:      defaultColorThreshold,
	}
}

func (p *LocalProvider) Name() string { return "local" }

func (p *LocalProvider) SupportedModes() []backgroundremoval.BackgroundMode {
	return []backgroundremoval.BackgroundMode{
		backgroundremoval.ModeKeepAlpha,
		backgroundremoval.ModeRemoveBackground,
		backgroundremoval.ModeUseExistingAlpha,
	}
}

func (p *LocalProvider) RemoveBackground(ctx context.Context, input backgroundremoval.ImageInput) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if input.Image == nil {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "input image is nil",
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}

	bounds := input.Image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "input image has zero size",
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}

	inputWidth := input.Width
	if inputWidth <= 0 {
		inputWidth = width
	}
	inputHeight := input.Height
	if inputHeight <= 0 {
		inputHeight = height
	}

	switch input.Mode {
	case backgroundremoval.ModeKeepAlpha:
		return p.handleKeepAlpha(ctx, input.Image, inputWidth, inputHeight)
	case backgroundremoval.ModeUseExistingAlpha:
		return p.handleUseExistingAlpha(ctx, input.Image, inputWidth, inputHeight)
	case backgroundremoval.ModeRemoveBackground:
		return p.handleRemoveBackground(ctx, input.Image, inputWidth, inputHeight)
	default:
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "unsupported mode: " + string(input.Mode),
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}
}

func (p *LocalProvider) handleKeepAlpha(ctx context.Context, img image.Image, inputW, inputH int) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	box := computeAlphaBounds(img, p.alphaThreshold)
	return &backgroundremoval.BackgroundRemovalResult{
		Image:      img,
		Width:      inputW,
		Height:     inputH,
		SubjectBox: box,
		AlphaValid: !box.Empty,
	}, nil
}

func (p *LocalProvider) handleUseExistingAlpha(ctx context.Context, img image.Image, inputW, inputH int) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if isFullyTransparent(img) {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeAlphaChannelInvalid,
			Message: "image alpha channel is fully transparent",
			Err:     backgroundremoval.ErrAlphaChannelInvalid,
		}
	}

	box := p.detectSubjectBox(img)
	if box.Empty {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeSubjectNotFound,
			Message: "no subject detected in image",
			Err:     backgroundremoval.ErrSubjectNotFound,
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cleaned := cleanAlphaEdges(img, p.alphaThreshold, p.edgeErodeRadius)

	return &backgroundremoval.BackgroundRemovalResult{
		Image:      cleaned,
		Width:      inputW,
		Height:     inputH,
		SubjectBox: box,
		AlphaValid: true,
	}, nil
}

func (p *LocalProvider) handleRemoveBackground(ctx context.Context, img image.Image, inputW, inputH int) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var workImage image.Image = img

	if !hasAlpha(img) {
		bgR, bgG, bgB := detectBackgroundColor(img)
		workImage = removeByColor(img, bgR, bgG, bgB, p.colorThreshold)
	} else if isFullyTransparent(img) {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeAlphaChannelInvalid,
			Message: "image alpha channel is fully transparent",
			Err:     backgroundremoval.ErrAlphaChannelInvalid,
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	box := p.detectSubjectBox(workImage)
	if box.Empty {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeSubjectNotFound,
			Message: "no subject detected in image",
			Err:     backgroundremoval.ErrSubjectNotFound,
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cleaned := cleanAlphaEdges(workImage, p.alphaThreshold, p.edgeErodeRadius)

	return &backgroundremoval.BackgroundRemovalResult{
		Image:      cleaned,
		Width:      inputW,
		Height:     inputH,
		SubjectBox: box,
		AlphaValid: true,
	}, nil
}

func (p *LocalProvider) detectSubjectBox(img image.Image) backgroundremoval.SubjectBox {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	mask := make([][]bool, h)
	for i := range mask {
		mask[i] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if c.A >= p.alphaThreshold {
				mask[y][x] = true
			}
		}
	}

	minArea := int(float64(w*h) * p.minSubjectAreaRatio)
	if minArea < 1 {
		minArea = 1
	}

	filteredMask := connectedComponents(mask, minArea)

	minX, minY := w, h
	maxX, maxY := -1, -1
	found := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if filteredMask[y][x] {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
				found = true
			}
		}
	}

	if !found {
		return backgroundremoval.SubjectBox{Empty: true}
	}

	return backgroundremoval.SubjectBox{
		MinX:   bounds.Min.X + minX,
		MinY:   bounds.Min.Y + minY,
		MaxX:   bounds.Min.X + maxX,
		MaxY:   bounds.Min.Y + maxY,
		Width:  maxX - minX + 1,
		Height: maxY - minY + 1,
		Empty:  false,
	}
}

func computeAlphaBounds(img image.Image, alphaThreshold uint8) backgroundremoval.SubjectBox {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A >= alphaThreshold {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
				found = true
			}
		}
	}
	if !found {
		return backgroundremoval.SubjectBox{Empty: true}
	}
	return backgroundremoval.SubjectBox{
		MinX:   minX,
		MinY:   minY,
		MaxX:   maxX,
		MaxY:   maxY,
		Width:  maxX - minX + 1,
		Height: maxY - minY + 1,
		Empty:  false,
	}
}

func connectedComponents(mask [][]bool, minArea int) [][]bool {
	h := len(mask)
	if h == 0 {
		return mask
	}
	w := len(mask[0])
	if w == 0 {
		return mask
	}

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}

	result := make([][]bool, h)
	for i := range result {
		result[i] = make([]bool, w)
	}

	type pixel struct{ x, y int }

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] || visited[y][x] {
				continue
			}

			var pixels []pixel
			queue := []pixel{{x, y}}
			visited[y][x] = true

			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				pixels = append(pixels, p)

				neighbors := [4]pixel{
					{p.x - 1, p.y}, {p.x + 1, p.y},
					{p.x, p.y - 1}, {p.x, p.y + 1},
				}
				for _, n := range neighbors {
					if n.x < 0 || n.x >= w || n.y < 0 || n.y >= h {
						continue
					}
					if !mask[n.y][n.x] || visited[n.y][n.x] {
						continue
					}
					visited[n.y][n.x] = true
					queue = append(queue, n)
				}
			}

			if len(pixels) >= minArea {
				for _, p := range pixels {
					result[p.y][p.x] = true
				}
			}
		}
	}

	return result
}

func cleanAlphaEdges(img image.Image, alphaThreshold uint8, radius int) image.Image {
	if radius <= 0 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	src := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			src.Set(x, y, img.At(x, y))
		}
	}

	mask := make([][]bool, h)
	for i := range mask {
		mask[i] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := src.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			if src.Pix[idx+3] >= alphaThreshold {
				mask[y][x] = true
			}
		}
	}

	opened := dilate(erode(mask, radius), radius)

	result := image.NewNRGBA(bounds)
	copy(result.Pix, src.Pix)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !opened[y][x] {
				idx := result.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
				result.Pix[idx+3] = 0
			}
		}
	}

	return result
}

func erode(mask [][]bool, radius int) [][]bool {
	h := len(mask)
	if h == 0 {
		return mask
	}
	w := len(mask[0])
	if w == 0 {
		return mask
	}

	result := make([][]bool, h)
	for i := range result {
		result[i] = make([]bool, w)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] {
				continue
			}
			keep := true
			for dy := -radius; dy <= radius && keep; dy++ {
				for dx := -radius; dx <= radius && keep; dx++ {
					ny, nx := y+dy, x+dx
					if ny < 0 || ny >= h || nx < 0 || nx >= w {
						keep = false
					} else if !mask[ny][nx] {
						keep = false
					}
				}
			}
			result[y][x] = keep
		}
	}

	return result
}

func dilate(mask [][]bool, radius int) [][]bool {
	h := len(mask)
	if h == 0 {
		return mask
	}
	w := len(mask[0])
	if w == 0 {
		return mask
	}

	result := make([][]bool, h)
	for i := range result {
		result[i] = make([]bool, w)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] {
				continue
			}
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					ny, nx := y+dy, x+dx
					if ny < 0 || ny >= h || nx < 0 || nx >= w {
						continue
					}
					result[ny][nx] = true
				}
			}
		}
	}

	return result
}

func detectBackgroundColor(img image.Image) (r, g, b uint8) {
	bounds := img.Bounds()

	corners := [4]struct{ x, y int }{
		{bounds.Min.X, bounds.Min.Y},
		{bounds.Max.X - 1, bounds.Min.Y},
		{bounds.Min.X, bounds.Max.Y - 1},
		{bounds.Max.X - 1, bounds.Max.Y - 1},
	}

	type colorKey struct{ r, g, b uint8 }
	counts := make(map[colorKey]int)

	for _, c := range corners {
		nc := color.NRGBAModel.Convert(img.At(c.x, c.y)).(color.NRGBA)
		key := colorKey{nc.R, nc.G, nc.B}
		counts[key]++
	}

	maxCount := 0
	var selected colorKey
	for key, count := range counts {
		if count > maxCount {
			maxCount = count
			selected = key
		}
	}

	return selected.r, selected.g, selected.b
}

func removeByColor(img image.Image, bgR, bgG, bgB uint8, threshold int) image.Image {
	bounds := img.Bounds()
	result := image.NewNRGBA(bounds)

	thresholdSq := threshold * threshold
	featherStart := int(float64(threshold) * 0.7)
	featherStartSq := featherStart * featherStart

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)

			dr := int(c.R) - int(bgR)
			dg := int(c.G) - int(bgG)
			db := int(c.B) - int(bgB)
			distSq := dr*dr + dg*dg + db*db

			var alpha uint8
			if distSq <= featherStartSq {
				alpha = 0
			} else if distSq <= thresholdSq {
				if thresholdSq == featherStartSq {
					alpha = 255
				} else {
					ratio := float64(distSq-featherStartSq) / float64(thresholdSq-featherStartSq)
					alpha = uint8(255 * ratio)
				}
			} else {
				alpha = 255
			}

			result.Set(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: alpha})
		}
	}

	return result
}

func hasAlpha(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return true
			}
		}
	}
	return false
}

func isFullyTransparent(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				return false
			}
		}
	}
	return true
}
