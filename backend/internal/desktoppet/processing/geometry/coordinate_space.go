// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

type CoordinateSpaceID string

const (
	SpaceSheetOriginal CoordinateSpaceID = "sheet_original"
	SpaceCellSource    CoordinateSpaceID = "cell_source"
	SpaceForeground    CoordinateSpaceID = "foreground"
	SpaceScaled        CoordinateSpaceID = "scaled"
	SpaceCanvas        CoordinateSpaceID = "canvas"
)

type PixelPoint struct {
	X, Y  float64
	Space CoordinateSpaceID
}

type NormalizedPoint struct {
	X, Y float64
}

type PixelRect struct {
	MinX, MinY, MaxX, MaxY int
	Space                  CoordinateSpaceID
}

func (r PixelRect) Width() int { return r.MaxX - r.MinX }

func (r PixelRect) Height() int { return r.MaxY - r.MinY }

func (r PixelRect) Area() int { return r.Width() * r.Height() }

func (r PixelRect) IsEmpty() bool { return r.MaxX <= r.MinX || r.MaxY <= r.MinY }

func (r PixelRect) Center() (float64, float64) {
	return float64(r.MinX+r.MaxX) / 2, float64(r.MinY+r.MaxY) / 2
}
