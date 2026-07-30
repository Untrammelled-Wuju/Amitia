// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"math"
)

type AffineTransform struct {
	From       CoordinateSpaceID
	To         CoordinateSpaceID
	M          [3][3]float64
	Invertible bool
	Reason     string
}

func NewScaleTransform(from, to CoordinateSpaceID, sx, sy float64) AffineTransform {
	return AffineTransform{
		From:       from,
		To:         to,
		M:          [3][3]float64{{sx, 0, 0}, {0, sy, 0}, {0, 0, 1}},
		Invertible: sx != 0 && sy != 0,
		Reason:     "scale",
	}
}

func NewTranslationTransform(from, to CoordinateSpaceID, tx, ty float64) AffineTransform {
	return AffineTransform{
		From:       from,
		To:         to,
		M:          [3][3]float64{{1, 0, tx}, {0, 1, ty}, {0, 0, 1}},
		Invertible: true,
		Reason:     "translation",
	}
}

func NewIdentityTransform(space CoordinateSpaceID) AffineTransform {
	return AffineTransform{
		From:       space,
		To:         space,
		M:          [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
		Invertible: true,
		Reason:     "identity",
	}
}

func (t AffineTransform) Apply(x, y float64) (float64, float64) {
	xn := t.M[0][0]*x + t.M[0][1]*y + t.M[0][2]
	yn := t.M[1][0]*x + t.M[1][1]*y + t.M[1][2]
	return xn, yn
}

func (t AffineTransform) ApplyRect(r PixelRect) PixelRect {
	x1 := float64(r.MinX)
	y1 := float64(r.MinY)
	x2 := float64(r.MaxX)
	y2 := float64(r.MaxY)

	corners := [4][2]float64{
		{x1, y1},
		{x2, y1},
		{x1, y2},
		{x2, y2},
	}

	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64

	for _, c := range corners {
		px, py := t.Apply(c[0], c[1])
		if px < minX {
			minX = px
		}
		if py < minY {
			minY = py
		}
		if px > maxX {
			maxX = px
		}
		if py > maxY {
			maxY = py
		}
	}

	return PixelRect{
		MinX:  int(math.Floor(minX)),
		MinY:  int(math.Floor(minY)),
		MaxX:  int(math.Ceil(maxX)),
		MaxY:  int(math.Ceil(maxY)),
		Space: t.To,
	}
}

func (t AffineTransform) Compose(other AffineTransform) AffineTransform {
	var result [3][3]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			sum := 0.0
			for k := 0; k < 3; k++ {
				sum += t.M[i][k] * other.M[k][j]
			}
			result[i][j] = sum
		}
	}
	return AffineTransform{
		From:       other.From,
		To:         t.To,
		M:          result,
		Invertible: t.Invertible && other.Invertible,
		Reason:     "compose",
	}
}

func (t AffineTransform) Inverse() (AffineTransform, error) {
	if !t.Invertible {
		return AffineTransform{}, errors.New("transform is not invertible: " + t.Reason)
	}

	a := t.M[0][0]
	b := t.M[0][1]
	tx := t.M[0][2]
	d := t.M[1][0]
	e := t.M[1][1]
	ty := t.M[1][2]

	det := a*e - b*d
	if math.Abs(det) < 1e-12 {
		return AffineTransform{}, errors.New("transform determinant is near zero")
	}

	invDet := 1.0 / det

	return AffineTransform{
		From: t.To,
		To:   t.From,
		M: [3][3]float64{
			{e * invDet, -b * invDet, (b*ty - e*tx) * invDet},
			{-d * invDet, a * invDet, (d*tx - a*ty) * invDet},
			{0, 0, 1},
		},
		Invertible: true,
		Reason:     "inverse",
	}, nil
}

func (t AffineTransform) IsIdentity() bool {
	return t.M[0][0] == 1 && t.M[0][1] == 0 && t.M[0][2] == 0 &&
		t.M[1][0] == 0 && t.M[1][1] == 1 && t.M[1][2] == 0 &&
		t.M[2][0] == 0 && t.M[2][1] == 0 && t.M[2][2] == 1
}
