// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package geometry

import (
	"errors"
	"image"

	"golang.org/x/image/draw"
)

type ResampleMode string

const (
	ResampleAuto         ResampleMode = "auto"
	ResampleIllustration ResampleMode = "illustration_high_quality"
	ResamplePixelArt     ResampleMode = "pixel_art_nearest"
)

type Resampler interface {
	ResizeRGBA(src *image.NRGBA, width, height int) (*image.NRGBA, error)
	ResizeMask(src *image.Gray, width, height int) (*image.Gray, error)
}

func NewResampler(mode ResampleMode) Resampler {
	switch mode {
	case ResamplePixelArt:
		return &pixelArtResampler{}
	case ResampleIllustration:
		return &illustrationResampler{}
	case ResampleAuto:
		return &autoResampler{}
	default:
		return &autoResampler{}
	}
}

type illustrationResampler struct{}

func (r *illustrationResampler) ResizeRGBA(src *image.NRGBA, width, height int) (*image.NRGBA, error) {
	if src == nil {
		return nil, errors.New("source image is nil")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid dimensions")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}

func (r *illustrationResampler) ResizeMask(src *image.Gray, width, height int) (*image.Gray, error) {
	if src == nil {
		return nil, errors.New("source mask is nil")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid dimensions")
	}
	dst := image.NewGray(image.Rect(0, 0, width, height))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}

type pixelArtResampler struct{}

func (r *pixelArtResampler) ResizeRGBA(src *image.NRGBA, width, height int) (*image.NRGBA, error) {
	if src == nil {
		return nil, errors.New("source image is nil")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid dimensions")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}

func (r *pixelArtResampler) ResizeMask(src *image.Gray, width, height int) (*image.Gray, error) {
	if src == nil {
		return nil, errors.New("source mask is nil")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid dimensions")
	}
	dst := image.NewGray(image.Rect(0, 0, width, height))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst, nil
}

type autoResampler struct {
	illustrationResampler
}
