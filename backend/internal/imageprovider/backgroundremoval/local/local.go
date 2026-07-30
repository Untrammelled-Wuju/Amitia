// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package local

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

const (
	defaultMinSubjectAreaRatio = 0.005
	defaultAlphaThreshold      = 10
	defaultEdgeErodeRadius     = 1
	defaultColorThreshold      = 30
	defaultSatelliteAreaRatio  = 0.02
	defaultCornerRegionPercent = 0.03
	maxCornerRegionSize        = 20
	colorSimilarityThreshold   = 30
)

type LocalProvider struct {
	minSubjectAreaRatio float64
	alphaThreshold      uint8
	edgeErodeRadius     int
	colorThreshold      int
	satelliteAreaRatio  float64
}

func NewLocalProvider() *LocalProvider {
	return &LocalProvider{
		minSubjectAreaRatio: defaultMinSubjectAreaRatio,
		alphaThreshold:      defaultAlphaThreshold,
		edgeErodeRadius:     defaultEdgeErodeRadius,
		colorThreshold:      defaultColorThreshold,
		satelliteAreaRatio:  defaultSatelliteAreaRatio,
	}
}

func NewLocalProviderWithCapabilities() (*LocalProvider, backgroundremoval.BackgroundRemovalCapabilities) {
	return NewLocalProvider(), LocalCapabilities()
}

func LocalCapabilities() backgroundremoval.BackgroundRemovalCapabilities {
	return backgroundremoval.BackgroundRemovalCapabilities{
		ProviderName:    "local-color-key",
		ProviderVersion: "v1",
		SupportedModes: []backgroundremoval.BackgroundMode{
			backgroundremoval.ModeKeepAlpha,
			backgroundremoval.ModeRemoveBackground,
			backgroundremoval.ModeUseExistingAlpha,
		},
		SupportedMIMEs:       []string{"image/png", "image/jpeg", "image/webp"},
		MaxWidth:             8192,
		MaxHeight:            8192,
		MaxPixels:            67108864,
		ReturnsMask:          true,
		PreservesSemiAlpha:   true,
		SupportsBatch:        false,
		SupportsCancellation: true,
		NetworkRequired:      false,
	}
}

func (p *LocalProvider) Name() string { return "local-color-key" }

func (p *LocalProvider) SupportedModes() []backgroundremoval.BackgroundMode {
	return []backgroundremoval.BackgroundMode{
		backgroundremoval.ModeKeepAlpha,
		backgroundremoval.ModeRemoveBackground,
		backgroundremoval.ModeUseExistingAlpha,
	}
}

func (p *LocalProvider) Capabilities() backgroundremoval.BackgroundRemovalCapabilities {
	return LocalCapabilities()
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

	nrgbaImg := toNRGBA(input.Image)

	req := backgroundremoval.BackgroundRemovalRequest{
		RequestID: fmt.Sprintf("v1-%d", time.Now().UnixNano()),
		Image:     nrgbaImg,
		Mode:      input.Mode,
	}

	result, err := p.RemoveBackgroundV2(ctx, req)
	if err != nil {
		return nil, err
	}

	if input.Width > 0 {
		result.Width = input.Width
	}
	if input.Height > 0 {
		result.Height = input.Height
	}

	return result, nil
}

func (p *LocalProvider) RemoveBackgroundV2(ctx context.Context, req backgroundremoval.BackgroundRemovalRequest) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if req.Image == nil {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "input image is nil",
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	bounds := req.Image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "input image has zero size",
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}

	alphaStats := analyzeAlphaDistribution(req.Image)

	switch req.Mode {
	case backgroundremoval.ModeKeepAlpha:
		return p.handleKeepAlphaV2(ctx, req.Image, bounds, width, height)
	case backgroundremoval.ModeUseExistingAlpha:
		return p.handleUseExistingAlphaV2(ctx, req.Image, bounds, width, height, alphaStats)
	case backgroundremoval.ModeRemoveBackground:
		return p.handleRemoveBackgroundV2(ctx, req.Image, bounds, width, height, alphaStats)
	default:
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeBackgroundRemovalFailed,
			Message: "unsupported mode: " + string(req.Mode),
			Err:     backgroundremoval.ErrBackgroundRemovalFailed,
		}
	}
}

func (p *LocalProvider) handleKeepAlphaV2(ctx context.Context, img *image.NRGBA, bounds image.Rectangle, width, height int) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	box := computeAlphaBounds(img, p.alphaThreshold)
	mask := alphaToGrayMask(img, bounds)

	return &backgroundremoval.BackgroundRemovalResult{
		Foreground:   img,
		Mask:         mask,
		Provider:     p.Name(),
		Degraded:     false,
		Measurements: backgroundremoval.BackgroundMeasurements{},
		Image:        img,
		Width:        width,
		Height:       height,
		SubjectBox:   box,
		AlphaValid:   !box.Empty,
	}, nil
}

func (p *LocalProvider) handleUseExistingAlphaV2(ctx context.Context, img *image.NRGBA, bounds image.Rectangle, width, height int, alphaStats alphaDistribution) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if alphaStats.AllTransparent {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeAlphaChannelInvalid,
			Message: "image alpha channel is fully transparent",
			Err:     backgroundremoval.ErrAlphaChannelInvalid,
		}
	}

	if alphaStats.AllOpaque {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeAlphaChannelInvalid,
			Message: "use_existing_alpha requires non-opaque alpha channel",
			Err:     backgroundremoval.ErrAlphaChannelInvalid,
		}
	}

	workImage := cleanAlphaEdgesNRGBA(img, p.alphaThreshold, p.edgeErodeRadius)

	return p.finalizeResult(ctx, workImage, bounds, width, height, backgroundremoval.BackgroundMeasurements{})
}

func (p *LocalProvider) handleRemoveBackgroundV2(ctx context.Context, img *image.NRGBA, bounds image.Rectangle, width, height int, alphaStats alphaDistribution) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if alphaStats.AllTransparent {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeAlphaChannelInvalid,
			Message: "image alpha channel is fully transparent",
			Err:     backgroundremoval.ErrAlphaChannelInvalid,
		}
	}

	var workImage *image.NRGBA
	measurements := backgroundremoval.BackgroundMeasurements{}

	if alphaStats.AllOpaque || !alphaStats.HasAnyAlpha {
		bgR, bgG, bgB, consistency, variance, _ := detectBackgroundColorRegions(img)
		measurements.CornerConsistency = consistency
		measurements.BackgroundVariance = variance
		workImage = removeByColorNRGBA(img, bgR, bgG, bgB, p.colorThreshold)
	} else {
		workImage = cleanAlphaEdgesNRGBA(img, p.alphaThreshold, p.edgeErodeRadius)
	}

	return p.finalizeResult(ctx, workImage, bounds, width, height, measurements)
}

func (p *LocalProvider) finalizeResult(ctx context.Context, workImage *image.NRGBA, bounds image.Rectangle, width, height int, measurements backgroundremoval.BackgroundMeasurements) (*backgroundremoval.BackgroundRemovalResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	boolMask := alphaToBoolMask(workImage, p.alphaThreshold, bounds)
	filteredMask := connectedComponentsWithSatellites(boolMask, width, height, p.minSubjectAreaRatio, p.satelliteAreaRatio)

	subjectBox := computeMaskBounds(filteredMask, bounds)
	if subjectBox.Empty {
		return nil, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeSubjectNotFound,
			Message: "no subject detected in image",
			Err:     backgroundremoval.ErrSubjectNotFound,
		}
	}

	grayMask := createGrayMaskFromAlpha(workImage, filteredMask, bounds)
	foreground := applyFilteredMask(workImage, filteredMask, bounds)

	measurements.RemovedRatio = computeRemovedRatio(filteredMask, width, height)
	measurements.BoundaryConnected = computeBoundaryConnected(filteredMask, width, height)
	measurements.Confidence = computeConfidence(measurements)

	return &backgroundremoval.BackgroundRemovalResult{
		Foreground:   foreground,
		Mask:         grayMask,
		Provider:     p.Name(),
		Degraded:     false,
		Measurements: measurements,
		Image:        foreground,
		Width:        width,
		Height:       height,
		SubjectBox:   subjectBox,
		AlphaValid:   true,
	}, nil
}

type alphaDistribution struct {
	Total           int
	Opaque          int
	Transparent     int
	Partial         int
	AllOpaque       bool
	AllTransparent  bool
	HasPartialAlpha bool
	HasAnyAlpha     bool
}

func analyzeAlphaDistribution(img image.Image) alphaDistribution {
	bounds := img.Bounds()
	dist := alphaDistribution{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			alpha8 := uint8(a >> 8)
			dist.Total++
			if alpha8 == 255 {
				dist.Opaque++
			} else if alpha8 == 0 {
				dist.Transparent++
			} else {
				dist.Partial++
			}
		}
	}
	dist.AllOpaque = dist.Total > 0 && dist.Opaque == dist.Total
	dist.AllTransparent = dist.Total > 0 && dist.Transparent == dist.Total
	dist.HasPartialAlpha = dist.Partial > 0
	dist.HasAnyAlpha = !dist.AllOpaque
	return dist
}

func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

func alphaToGrayMask(img *image.NRGBA, bounds image.Rectangle) *image.Gray {
	mask := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			idx := img.PixOffset(x, y)
			mask.Pix[mask.PixOffset(x, y)] = img.Pix[idx+3]
		}
	}
	return mask
}

func alphaToBoolMask(img *image.NRGBA, threshold uint8, bounds image.Rectangle) [][]bool {
	w := bounds.Dx()
	h := bounds.Dy()
	mask := make([][]bool, h)
	for i := range mask {
		mask[i] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			if img.Pix[idx+3] >= threshold {
				mask[y][x] = true
			}
		}
	}
	return mask
}

func createGrayMaskFromAlpha(img *image.NRGBA, filteredMask [][]bool, bounds image.Rectangle) *image.Gray {
	mask := image.NewGray(bounds)
	w := bounds.Dx()
	h := bounds.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if filteredMask[y][x] {
				idx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
				mask.Pix[mask.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)] = img.Pix[idx+3]
			}
		}
	}
	return mask
}

func applyFilteredMask(img *image.NRGBA, filteredMask [][]bool, bounds image.Rectangle) *image.NRGBA {
	w := bounds.Dx()
	h := bounds.Dy()
	result := image.NewNRGBA(bounds)
	copy(result.Pix, img.Pix)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !filteredMask[y][x] {
				idx := result.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
				result.Pix[idx+3] = 0
			}
		}
	}
	return result
}

func computeMaskBounds(mask [][]bool, bounds image.Rectangle) backgroundremoval.SubjectBox {
	h := len(mask)
	if h == 0 {
		return backgroundremoval.SubjectBox{Empty: true}
	}
	w := len(mask[0])
	if w == 0 {
		return backgroundremoval.SubjectBox{Empty: true}
	}

	minX, minY := w, h
	maxX, maxY := -1, -1
	found := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y][x] {
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

func computeRemovedRatio(mask [][]bool, w, h int) float64 {
	total := w * h
	if total == 0 {
		return 0
	}
	removed := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] {
				removed++
			}
		}
	}
	return float64(removed) / float64(total)
}

func computeBoundaryConnected(mask [][]bool, w, h int) float64 {
	if w == 0 || h == 0 {
		return 0
	}

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}

	totalKept := 0
	largestComponent := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] || visited[y][x] {
				continue
			}
			queue := [][2]int{{x, y}}
			visited[y][x] = true
			compSize := 0
			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				compSize++
				neighbors := [4][2]int{{p[0] - 1, p[1]}, {p[0] + 1, p[1]}, {p[0], p[1] - 1}, {p[0], p[1] + 1}}
				for _, n := range neighbors {
					nx, ny := n[0], n[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && mask[ny][nx] && !visited[ny][nx] {
						visited[ny][nx] = true
						queue = append(queue, [2]int{nx, ny})
					}
				}
			}
			totalKept += compSize
			if compSize > largestComponent {
				largestComponent = compSize
			}
		}
	}

	if totalKept == 0 {
		return 0
	}
	return float64(largestComponent) / float64(totalKept)
}

func computeConfidence(m backgroundremoval.BackgroundMeasurements) float64 {
	confidence := 0.0

	confidence += m.CornerConsistency * 0.3

	varianceScore := 1.0
	if m.BackgroundVariance > 0 {
		varianceScore = 1.0 / (1.0 + m.BackgroundVariance/100.0)
	}
	confidence += varianceScore * 0.3

	ratioScore := 1.0 - math.Abs(m.RemovedRatio-0.5)*2
	if ratioScore < 0 {
		ratioScore = 0
	}
	confidence += ratioScore * 0.2

	confidence += m.BoundaryConnected * 0.2

	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return confidence
}

type cornerRegion struct {
	MedianR  uint8
	MedianG  uint8
	MedianB  uint8
	Variance float64
}

func computeCornerRegion(img image.Image, x0, y0, x1, y1 int) cornerRegion {
	type sample struct{ r, g, b uint8 }
	var samples []sample

	bounds := img.Bounds()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
				continue
			}
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			samples = append(samples, sample{c.R, c.G, c.B})
		}
	}

	if len(samples) == 0 {
		return cornerRegion{}
	}

	rs := make([]uint8, len(samples))
	gs := make([]uint8, len(samples))
	bs := make([]uint8, len(samples))
	for i, s := range samples {
		rs[i] = s.r
		gs[i] = s.g
		bs[i] = s.b
	}

	sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
	sort.Slice(gs, func(i, j int) bool { return gs[i] < gs[j] })
	sort.Slice(bs, func(i, j int) bool { return bs[i] < bs[j] })

	medianIdx := len(samples) / 2
	medianR := rs[medianIdx]
	medianG := gs[medianIdx]
	medianB := bs[medianIdx]

	var sumSqDiff float64
	for _, s := range samples {
		dr := float64(s.r) - float64(medianR)
		dg := float64(s.g) - float64(medianG)
		db := float64(s.b) - float64(medianB)
		sumSqDiff += dr*dr + dg*dg + db*db
	}
	variance := sumSqDiff / float64(len(samples))

	return cornerRegion{
		MedianR:  medianR,
		MedianG:  medianG,
		MedianB:  medianB,
		Variance: variance,
	}
}

func colorsSimilar(r1, g1, b1, r2, g2, b2 uint8, threshold int) bool {
	dr := int(r1) - int(r2)
	dg := int(g1) - int(g2)
	db := int(b1) - int(b2)
	distSq := dr*dr + dg*dg + db*db
	return distSq <= threshold*threshold
}

func detectBackgroundColorRegions(img image.Image) (r, g, b uint8, consistency float64, variance float64, ok bool) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	shortSide := w
	if h < shortSide {
		shortSide = h
	}
	regionSize := int(float64(shortSide) * defaultCornerRegionPercent)
	if regionSize < 1 {
		regionSize = 1
	}
	if regionSize > maxCornerRegionSize {
		regionSize = maxCornerRegionSize
	}

	corners := [4]struct{ x0, y0, x1, y1 int }{
		{bounds.Min.X, bounds.Min.Y, bounds.Min.X + regionSize, bounds.Min.Y + regionSize},
		{bounds.Max.X - regionSize, bounds.Min.Y, bounds.Max.X, bounds.Min.Y + regionSize},
		{bounds.Min.X, bounds.Max.Y - regionSize, bounds.Min.X + regionSize, bounds.Max.Y},
		{bounds.Max.X - regionSize, bounds.Max.Y - regionSize, bounds.Max.X, bounds.Max.Y},
	}

	regions := make([]cornerRegion, 4)
	for i, c := range corners {
		regions[i] = computeCornerRegion(img, c.x0, c.y0, c.x1, c.y1)
	}

	bestCount := 0
	bestIdx := 0
	for i := 0; i < 4; i++ {
		count := 1
		for j := 0; j < 4; j++ {
			if i == j {
				continue
			}
			if colorsSimilar(regions[i].MedianR, regions[i].MedianG, regions[i].MedianB,
				regions[j].MedianR, regions[j].MedianG, regions[j].MedianB, colorSimilarityThreshold) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			bestIdx = i
		}
	}

	consistency = float64(bestCount) / 4.0

	totalVariance := 0.0
	for _, reg := range regions {
		totalVariance += reg.Variance
	}
	variance = totalVariance / 4.0

	ok = bestCount >= 3
	if !ok {
		ok = bestCount >= 2
	}

	r = regions[bestIdx].MedianR
	g = regions[bestIdx].MedianG
	b = regions[bestIdx].MedianB
	return
}

func connectedComponentsWithSatellites(mask [][]bool, w, h int, minAreaRatio, satelliteRatio float64) [][]bool {
	if h == 0 || w == 0 {
		return mask
	}

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}

	type component struct {
		pixels [][2]int
		size   int
	}
	var components []component

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y][x] || visited[y][x] {
				continue
			}

			var pixels [][2]int
			queue := [][2]int{{x, y}}
			visited[y][x] = true

			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				pixels = append(pixels, p)

				neighbors := [4][2]int{{p[0] - 1, p[1]}, {p[0] + 1, p[1]}, {p[0], p[1] - 1}, {p[0], p[1] + 1}}
				for _, n := range neighbors {
					nx, ny := n[0], n[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && mask[ny][nx] && !visited[ny][nx] {
						visited[ny][nx] = true
						queue = append(queue, [2]int{nx, ny})
					}
				}
			}

			components = append(components, component{pixels: pixels, size: len(pixels)})
		}
	}

	result := make([][]bool, h)
	for i := range result {
		result[i] = make([]bool, w)
	}

	if len(components) == 0 {
		return result
	}

	minArea := int(float64(w*h) * minAreaRatio)
	if minArea < 1 {
		minArea = 1
	}

	largestSize := 0
	for _, c := range components {
		if c.size > largestSize {
			largestSize = c.size
		}
	}

	satelliteMinArea := int(float64(largestSize) * satelliteRatio)
	if satelliteMinArea < 1 {
		satelliteMinArea = 1
	}

	hasMainSubject := false
	for _, c := range components {
		if c.size >= minArea {
			hasMainSubject = true
			break
		}
	}

	if !hasMainSubject {
		return result
	}

	for _, c := range components {
		keep := false
		if c.size >= minArea {
			keep = true
		} else if c.size >= satelliteMinArea {
			keep = true
		}

		if keep {
			for _, p := range c.pixels {
				result[p[1]][p[0]] = true
			}
		}
	}

	return result
}

func cleanAlphaEdgesNRGBA(img *image.NRGBA, alphaThreshold uint8, radius int) *image.NRGBA {
	if radius <= 0 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	mask := make([][]bool, h)
	for i := range mask {
		mask[i] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			if img.Pix[idx+3] >= alphaThreshold {
				mask[y][x] = true
			}
		}
	}

	opened := dilate(erode(mask, radius), radius)

	result := image.NewNRGBA(bounds)
	copy(result.Pix, img.Pix)
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

func removeByColorNRGBA(img *image.NRGBA, bgR, bgG, bgB uint8, threshold int) *image.NRGBA {
	bounds := img.Bounds()
	result := image.NewNRGBA(bounds)

	thresholdSq := threshold * threshold
	featherStart := int(float64(threshold) * 0.7)
	featherStartSq := featherStart * featherStart

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			idx := img.PixOffset(x, y)
			r := img.Pix[idx]
			g := img.Pix[idx+1]
			b := img.Pix[idx+2]

			dr := int(r) - int(bgR)
			dg := int(g) - int(bgG)
			db := int(b) - int(bgB)
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

			dstIdx := result.PixOffset(x, y)
			result.Pix[dstIdx] = r
			result.Pix[dstIdx+1] = g
			result.Pix[dstIdx+2] = b
			result.Pix[dstIdx+3] = alpha
		}
	}

	return result
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
	r, g, b, _, _, _ = detectBackgroundColorRegions(img)
	return
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
	return analyzeAlphaDistribution(img).HasAnyAlpha
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
