// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func newTestImage(w, h int, fill color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, fill)
		}
	}
	return img
}

func drawRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	b := img.Bounds()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || x >= b.Dx() || y < 0 || y >= b.Dy() {
				continue
			}
			img.Set(x, y, c)
		}
	}
}

func TestSubjectDetectAlpha(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	drawRect(img, 30, 20, 70, 80, color.NRGBA{255, 255, 255, 255})

	d := NewSubjectDetector()
	box, err := d.DetectSubject(img)
	if err != nil {
		t.Fatalf("DetectSubject failed: %v", err)
	}
	if box.Empty {
		t.Fatal("box should not be empty")
	}
	if box.MinX != 30 || box.MinY != 20 || box.MaxX != 69 || box.MaxY != 79 {
		t.Errorf("unexpected box: %+v", box)
	}
	if box.Width != 40 || box.Height != 60 {
		t.Errorf("unexpected dimensions: w=%d h=%d", box.Width, box.Height)
	}
}

func TestSubjectDetectNoiseFiltered(t *testing.T) {
	img := newTestImage(100, 100, color.NRGBA{0, 0, 0, 0})
	img.Set(5, 5, color.NRGBA{255, 255, 255, 255})
	img.Set(6, 6, color.NRGBA{255, 255, 255, 255})

	d := NewSubjectDetector()
	box, err := d.DetectSubject(img)
	if err == nil {
		t.Fatalf("expected subject not found error, got box %+v", box)
	}
	if !errors.Is(err, backgroundremoval.ErrSubjectNotFound) {
		t.Errorf("expected ErrSubjectNotFound, got %v", err)
	}
}

func TestSubjectDetectFullyTransparent(t *testing.T) {
	img := newTestImage(80, 80, color.NRGBA{0, 0, 0, 0})
	d := NewSubjectDetector()
	_, err := d.DetectSubject(img)
	if err == nil {
		t.Fatal("expected error for fully transparent image")
	}
}

func TestSubjectDetectSubjectsBatch(t *testing.T) {
	img1 := newTestImage(80, 80, color.NRGBA{0, 0, 0, 0})
	drawRect(img1, 10, 10, 70, 70, color.NRGBA{255, 0, 0, 255})
	img2 := newTestImage(80, 80, color.NRGBA{0, 0, 0, 0})
	drawRect(img2, 20, 20, 60, 60, color.NRGBA{0, 255, 0, 255})

	d := NewSubjectDetector()
	boxes, err := d.DetectSubjects([]image.Image{img1, img2})
	if err != nil {
		t.Fatalf("DetectSubjects failed: %v", err)
	}
	if len(boxes) != 2 {
		t.Fatalf("expected 2 boxes, got %d", len(boxes))
	}
	if boxes[0].Empty || boxes[1].Empty {
		t.Errorf("boxes should not be empty: %+v %+v", boxes[0], boxes[1])
	}
	if boxes[0].MinX != 10 || boxes[0].MaxY != 69 {
		t.Errorf("unexpected first box: %+v", boxes[0])
	}
	if boxes[1].MinX != 20 || boxes[1].MaxY != 59 {
		t.Errorf("unexpected second box: %+v", boxes[1])
	}
}

func TestSubjectDetectNilImage(t *testing.T) {
	d := NewSubjectDetector()
	_, err := d.DetectSubject(nil)
	if err == nil {
		t.Fatal("expected error for nil image")
	}
}

func TestSubjectMaxBox(t *testing.T) {
	boxes := []backgroundremoval.SubjectBox{
		{MinX: 10, MinY: 10, MaxX: 50, MaxY: 40, Width: 41, Height: 31, Empty: false},
		{MinX: 5, MinY: 20, MaxX: 60, MaxY: 70, Width: 56, Height: 51, Empty: false},
		{Empty: true},
	}
	m := MaxSubjectBox(boxes)
	if m.Empty {
		t.Fatal("max box should not be empty")
	}
	if m.MinX != 5 || m.MinY != 10 || m.MaxX != 60 || m.MaxY != 70 {
		t.Errorf("unexpected max box: %+v", m)
	}
	if m.Width != 56 || m.Height != 61 {
		t.Errorf("unexpected max box dimensions: w=%d h=%d", m.Width, m.Height)
	}
}

func TestSubjectMaxBoxAllEmpty(t *testing.T) {
	boxes := []backgroundremoval.SubjectBox{{Empty: true}, {Empty: true}}
	m := MaxSubjectBox(boxes)
	if !m.Empty {
		t.Errorf("expected empty max box, got %+v", m)
	}
}

func TestSubjectMaxBoxEmpty(t *testing.T) {
	m := MaxSubjectBox(nil)
	if !m.Empty {
		t.Errorf("expected empty max box for nil input")
	}
}
