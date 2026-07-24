// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"errors"
	"image"
	"image/color"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

const (
	defaultAlphaThreshold      = 10
	defaultMinSubjectAreaRatio = 0.005
)

type SubjectDetector struct {
	minAreaRatio   float64
	alphaThreshold uint8
}

func NewSubjectDetector() *SubjectDetector {
	return &SubjectDetector{
		minAreaRatio:   defaultMinSubjectAreaRatio,
		alphaThreshold: defaultAlphaThreshold,
	}
}

func (d *SubjectDetector) DetectSubject(img image.Image) (backgroundremoval.SubjectBox, error) {
	if img == nil {
		return backgroundremoval.SubjectBox{Empty: true}, errors.New("input image is nil")
	}
	box := d.detectBox(img)
	if box.Empty {
		return backgroundremoval.SubjectBox{Empty: true}, &backgroundremoval.ProviderError{
			Code:    backgroundremoval.ErrCodeSubjectNotFound,
			Message: "no subject detected in image",
			Err:     backgroundremoval.ErrSubjectNotFound,
		}
	}
	return box, nil
}

func (d *SubjectDetector) DetectSubjects(imgs []image.Image) ([]backgroundremoval.SubjectBox, error) {
	boxes := make([]backgroundremoval.SubjectBox, 0, len(imgs))
	for _, img := range imgs {
		box, err := d.DetectSubject(img)
		if err != nil {
			return nil, err
		}
		boxes = append(boxes, box)
	}
	return boxes, nil
}

func (d *SubjectDetector) detectBox(img image.Image) backgroundremoval.SubjectBox {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return backgroundremoval.SubjectBox{Empty: true}
	}

	mask := make([][]bool, h)
	for i := range mask {
		mask[i] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if c.A >= d.alphaThreshold {
				mask[y][x] = true
			}
		}
	}

	minArea := int(float64(w*h) * d.minAreaRatio)
	if minArea < 1 {
		minArea = 1
	}

	filtered := connectedComponents(mask, minArea)

	minX, minY := w, h
	maxX, maxY := -1, -1
	found := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !filtered[y][x] {
				continue
			}
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

func MaxSubjectBox(boxes []backgroundremoval.SubjectBox) backgroundremoval.SubjectBox {
	if len(boxes) == 0 {
		return backgroundremoval.SubjectBox{Empty: true}
	}

	minX := int(^uint(0) >> 1)
	minY := int(^uint(0) >> 1)
	maxX := -1
	maxY := -1
	any := false
	for _, b := range boxes {
		if b.Empty {
			continue
		}
		any = true
		if b.MinX < minX {
			minX = b.MinX
		}
		if b.MinY < minY {
			minY = b.MinY
		}
		if b.MaxX > maxX {
			maxX = b.MaxX
		}
		if b.MaxY > maxY {
			maxY = b.MaxY
		}
	}

	if !any {
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
