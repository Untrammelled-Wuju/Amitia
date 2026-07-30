// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"sort"
)

type SubjectAnalysis struct {
	AlphaCoverage     float64
	SubjectBox        PixelRect
	Components        []ComponentInfo
	MainComponent     *ComponentInfo
	EdgeContact       EdgeContactInfo
	HoleRatio         float64
	MaskHash          string
	TotalOpaquePixels int
}

type ComponentInfo struct {
	Area        int
	Box         PixelRect
	CentroidX   float64
	CentroidY   float64
	TouchesEdge bool
	IsMain      bool
}

type EdgeContactInfo struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
	Count  int
}

var neighbors4 = [4][2]int{{0, -1}, {-1, 0}, {1, 0}, {0, 1}}

var neighbors8 = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func AnalyzeMask(mask *image.Gray, threshold uint8, space CoordinateSpaceID, minAreaRatio float64, maxComponents int) (*SubjectAnalysis, error) {
	if mask == nil {
		return nil, errors.New("mask is nil")
	}

	bounds := mask.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, errors.New("mask has zero dimensions")
	}

	stride := mask.Stride
	pix := mask.Pix

	totalPixels := w * h
	opaque := make([]bool, totalPixels)
	totalOpaque := 0

	for y := 0; y < h; y++ {
		rowBase := y * stride
		for x := 0; x < w; x++ {
			if pix[rowBase+x] >= threshold {
				opaque[y*w+x] = true
				totalOpaque++
			}
		}
	}

	alphaCoverage := 0.0
	if totalPixels > 0 {
		alphaCoverage = float64(totalOpaque) / float64(totalPixels)
	}

	hashBytes := sha256.Sum256(mask.Pix)
	maskHash := hex.EncodeToString(hashBytes[:])

	visited := make([]bool, totalPixels)
	components := labelComponents(opaque, visited, w, h, true, space)

	if len(components) == 0 {
		return &SubjectAnalysis{
			AlphaCoverage:     alphaCoverage,
			SubjectBox:        PixelRect{Space: space},
			Components:        []ComponentInfo{},
			EdgeContact:       EdgeContactInfo{},
			HoleRatio:         0,
			MaskHash:          maskHash,
			TotalOpaquePixels: totalOpaque,
		}, nil
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].Area > components[j].Area
	})

	components[0].IsMain = true
	mainArea := components[0].Area

	minArea := int(float64(mainArea) * minAreaRatio)
	if minArea < 1 {
		minArea = 1
	}

	var kept []ComponentInfo
	for _, c := range components {
		if c.Area >= minArea {
			kept = append(kept, c)
		}
	}

	if maxComponents > 0 && len(kept) > maxComponents {
		kept = kept[:maxComponents]
	}

	subjectBox := PixelRect{Space: space}
	for i, c := range kept {
		if i == 0 {
			subjectBox = c.Box
		} else {
			if c.Box.MinX < subjectBox.MinX {
				subjectBox.MinX = c.Box.MinX
			}
			if c.Box.MinY < subjectBox.MinY {
				subjectBox.MinY = c.Box.MinY
			}
			if c.Box.MaxX > subjectBox.MaxX {
				subjectBox.MaxX = c.Box.MaxX
			}
			if c.Box.MaxY > subjectBox.MaxY {
				subjectBox.MaxY = c.Box.MaxY
			}
		}
	}

	edgeContact := EdgeContactInfo{}
	for _, c := range kept {
		if c.Box.MinY <= 0 {
			edgeContact.Top = true
		}
		if c.Box.MaxY >= h {
			edgeContact.Bottom = true
		}
		if c.Box.MinX <= 0 {
			edgeContact.Left = true
		}
		if c.Box.MaxX >= w {
			edgeContact.Right = true
		}
	}
	edgeContact.Count = 0
	if edgeContact.Top {
		edgeContact.Count++
	}
	if edgeContact.Bottom {
		edgeContact.Count++
	}
	if edgeContact.Left {
		edgeContact.Count++
	}
	if edgeContact.Right {
		edgeContact.Count++
	}

	opaqueInBox := 0
	for y := subjectBox.MinY; y < subjectBox.MaxY && y < h; y++ {
		if y < 0 {
			continue
		}
		for x := subjectBox.MinX; x < subjectBox.MaxX && x < w; x++ {
			if x < 0 {
				continue
			}
			if opaque[y*w+x] {
				opaqueInBox++
			}
		}
	}

	holeRatio := 0.0
	boxArea := subjectBox.Area()
	if boxArea > 0 {
		holeRatio = 1.0 - float64(opaqueInBox)/float64(boxArea)
	}

	var mainComp *ComponentInfo
	for i := range kept {
		if kept[i].IsMain {
			mainComp = &kept[i]
			break
		}
	}

	return &SubjectAnalysis{
		AlphaCoverage:     alphaCoverage,
		SubjectBox:        subjectBox,
		Components:        kept,
		MainComponent:     mainComp,
		EdgeContact:       edgeContact,
		HoleRatio:         holeRatio,
		MaskHash:          maskHash,
		TotalOpaquePixels: totalOpaque,
	}, nil
}

func labelComponents(opaque []bool, visited []bool, w, h int, use8Connectivity bool, space CoordinateSpaceID) []ComponentInfo {
	var neighbors [][2]int
	if use8Connectivity {
		neighbors = neighbors8[:]
	} else {
		neighbors = neighbors4[:]
	}

	var components []ComponentInfo

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !opaque[idx] || visited[idx] {
				continue
			}

			queue := make([]int, 0, 256)
			queue = append(queue, idx)
			visited[idx] = true

			area := 0
			minX, minY := x, y
			maxX, maxY := x, y
			sumX, sumY := 0.0, 0.0

			head := 0
			for head < len(queue) {
				curIdx := queue[head]
				head++
				cx := curIdx % w
				cy := curIdx / w
				area++
				sumX += float64(cx)
				sumY += float64(cy)

				if cx < minX {
					minX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cx > maxX {
					maxX = cx
				}
				if cy > maxY {
					maxY = cy
				}

				for _, n := range neighbors {
					nx := cx + n[0]
					ny := cy + n[1]
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nIdx := ny*w + nx
					if opaque[nIdx] && !visited[nIdx] {
						visited[nIdx] = true
						queue = append(queue, nIdx)
					}
				}
			}

			box := PixelRect{
				MinX:  minX,
				MinY:  minY,
				MaxX:  maxX + 1,
				MaxY:  maxY + 1,
				Space: space,
			}

			components = append(components, ComponentInfo{
				Area:        area,
				Box:         box,
				CentroidX:   sumX / float64(area),
				CentroidY:   sumY / float64(area),
				TouchesEdge: box.MinY <= 0 || box.MaxY >= h || box.MinX <= 0 || box.MaxX >= w,
			})
		}
	}

	return components
}
