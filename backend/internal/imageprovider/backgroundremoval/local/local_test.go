// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package local

import (
	"context"
	"errors"
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
)

func newSolidNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func newImageWithSubject(bg, subject color.NRGBA, w, h, subW, subH int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	offsetX := (w - subW) / 2
	offsetY := (h - subH) / 2
	for y := offsetY; y < offsetY+subH; y++ {
		for x := offsetX; x < offsetX+subW; x++ {
			img.Set(x, y, subject)
		}
	}
	return img
}

func newImageWithAlphaSubject(subject color.NRGBA, w, h, subW, subH int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	transparent := color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, transparent)
		}
	}
	offsetX := (w - subW) / 2
	offsetY := (h - subH) / 2
	for y := offsetY; y < offsetY+subH; y++ {
		for x := offsetX; x < offsetX+subW; x++ {
			img.Set(x, y, subject)
		}
	}
	return img
}

func TestLocalProvider_Name(t *testing.T) {
	p := NewLocalProvider()
	if p.Name() != "local-color-key" {
		t.Errorf("Name() = %s, want local-color-key", p.Name())
	}
}

func TestLocalProvider_SupportedModes(t *testing.T) {
	p := NewLocalProvider()
	modes := p.SupportedModes()
	if len(modes) != 3 {
		t.Fatalf("SupportedModes() length = %d, want 3", len(modes))
	}

	expected := map[backgroundremoval.BackgroundMode]bool{
		backgroundremoval.ModeKeepAlpha:        true,
		backgroundremoval.ModeRemoveBackground: true,
		backgroundremoval.ModeUseExistingAlpha: true,
	}
	for _, m := range modes {
		if !expected[m] {
			t.Errorf("unexpected mode: %s", m)
		}
	}
}

func TestLocalProvider_ModeKeepAlpha(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Image != src {
		t.Error("ModeKeepAlpha should return original image")
	}
	if result.Width != 20 || result.Height != 20 {
		t.Errorf("dimensions = %dx%d, want 20x20", result.Width, result.Height)
	}
	if result.SubjectBox.Empty {
		t.Error("SubjectBox should not be empty")
	}
	if !result.AlphaValid {
		t.Error("AlphaValid should be true")
	}
}

func TestLocalProvider_ModeKeepAlpha_NoAlpha(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(20, 20, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Image != src {
		t.Error("ModeKeepAlpha should return original image")
	}
	if result.SubjectBox.Empty {
		t.Error("SubjectBox should not be empty for fully opaque image")
	}
}

func TestLocalProvider_ModeUseExistingAlpha(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeUseExistingAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Image == nil {
		t.Fatal("result image is nil")
	}
	if result.SubjectBox.Empty {
		t.Error("SubjectBox should not be empty")
	}
	if !result.AlphaValid {
		t.Error("AlphaValid should be true")
	}
	if result.SubjectBox.Width <= 0 || result.SubjectBox.Height <= 0 {
		t.Errorf("SubjectBox dimensions invalid: %dx%d", result.SubjectBox.Width, result.SubjectBox.Height)
	}
}

func TestLocalProvider_ModeUseExistingAlpha_PreservesAlpha(t *testing.T) {
	p := NewLocalProvider()
	subject := color.NRGBA{R: 0, G: 255, B: 0, A: 200}
	src := newImageWithAlphaSubject(subject, 20, 20, 10, 10)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeUseExistingAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	bounds := result.Image.Bounds()
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2
	_, _, _, a := result.Image.At(centerX, centerY).RGBA()
	if a == 0 {
		t.Error("center pixel should not be transparent after use_existing_alpha")
	}
}

func TestLocalProvider_ModeRemoveBackground(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		40, 40, 20, 20,
	)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Image == nil {
		t.Fatal("result image is nil")
	}
	if result.SubjectBox.Empty {
		t.Error("SubjectBox should not be empty")
	}
	if !result.AlphaValid {
		t.Error("AlphaValid should be true")
	}

	_, _, _, a := result.Image.At(0, 0).RGBA()
	if a != 0 {
		t.Errorf("corner alpha = %d, want 0 (background should be transparent)", a>>8)
	}
}

func TestLocalProvider_ModeRemoveBackground_KeepsSubject(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		40, 40, 20, 20,
	)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	bounds := result.Image.Bounds()
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2
	_, _, _, a := result.Image.At(centerX, centerY).RGBA()
	if a == 0 {
		t.Error("center pixel should not be transparent (subject should be kept)")
	}
}

func TestLocalProvider_ModeRemoveBackground_WithAlpha(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Image == nil {
		t.Fatal("result image is nil")
	}
	if result.SubjectBox.Empty {
		t.Error("SubjectBox should not be empty")
	}
}

func TestLocalProvider_SubjectNotFound(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(40, 40, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeSubjectNotFound {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeSubjectNotFound)
	}
	if !errors.Is(err, backgroundremoval.ErrSubjectNotFound) {
		t.Error("error is not ErrSubjectNotFound")
	}
}

func TestLocalProvider_SubjectNotFound_UseExistingAlpha(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 100, 100, 5, 5)

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  100,
		Height: 100,
		Mode:   backgroundremoval.ModeUseExistingAlpha,
	})
	if err == nil {
		t.Fatal("expected error for tiny subject, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeSubjectNotFound {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeSubjectNotFound)
	}
}

func TestLocalProvider_AlphaChannelInvalid(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(20, 20, color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeUseExistingAlpha,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeAlphaChannelInvalid {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeAlphaChannelInvalid)
	}
	if !errors.Is(err, backgroundremoval.ErrAlphaChannelInvalid) {
		t.Error("error is not ErrAlphaChannelInvalid")
	}
}

func TestLocalProvider_AlphaChannelInvalid_RemoveBackground(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(20, 20, color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeAlphaChannelInvalid {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeAlphaChannelInvalid)
	}
}

func TestLocalProvider_ContextCancelled(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.RemoveBackground(ctx, backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error is not context.Canceled: %v", err)
	}
}

func TestLocalProvider_ContextTimeout(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
	defer cancel()

	_, err := p.RemoveBackground(ctx, backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error is not context.DeadlineExceeded: %v", err)
	}
}

func TestLocalProvider_ContextCancelled_RemoveBackground(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		40, 40, 20, 20,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.RemoveBackground(ctx, backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error is not context.Canceled: %v", err)
	}
}

func TestLocalProvider_NilImage(t *testing.T) {
	p := NewLocalProvider()

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  nil,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeBackgroundRemovalFailed {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeBackgroundRemovalFailed)
	}
}

func TestLocalProvider_UnsupportedMode(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(10, 10, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  10,
		Height: 10,
		Mode:   backgroundremoval.BackgroundMode("invalid"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *backgroundremoval.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != backgroundremoval.ErrCodeBackgroundRemovalFailed {
		t.Errorf("Code = %s, want %s", pe.Code, backgroundremoval.ErrCodeBackgroundRemovalFailed)
	}
}

func TestLocalProvider_NoNetworkFields(t *testing.T) {
	p := NewLocalProvider()

	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldInfo := v.Type().Field(i)
		typeName := field.Type().String()

		if strings.Contains(typeName, "http.") ||
			strings.Contains(typeName, "net.") ||
			strings.Contains(typeName, "url.") ||
			strings.Contains(typeName, "Client") {
			t.Errorf("LocalProvider field %s has network type %s", fieldInfo.Name, typeName)
		}
	}
}

func TestLocalProvider_NoNetworkCall(t *testing.T) {
	p := NewLocalProvider()

	src := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		40, 40, 20, 20,
	)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Image == nil {
		t.Fatal("result image is nil")
	}
}

func TestLocalProvider_DoesNotModifyOriginalImage(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		40, 40, 20, 20,
	)

	originalCorner := src.At(0, 0)

	_, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  40,
		Height: 40,
		Mode:   backgroundremoval.ModeRemoveBackground,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	afterCorner := src.At(0, 0)
	origR, origG, origB, origA := originalCorner.RGBA()
	afterR, afterG, afterB, afterA := afterCorner.RGBA()
	if origR != afterR || origG != afterG || origB != afterB || origA != afterA {
		t.Error("original image was modified by RemoveBackground")
	}
}

func TestLocalProvider_DefaultWidthHeight(t *testing.T) {
	p := NewLocalProvider()
	src := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  0,
		Height: 0,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if result.Width != 20 || result.Height != 20 {
		t.Errorf("dimensions = %dx%d, want 20x20 (from image bounds)", result.Width, result.Height)
	}
}

func TestLocalProvider_ModeKeepAlpha_FullyTransparent(t *testing.T) {
	p := NewLocalProvider()
	src := newSolidNRGBA(20, 20, color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeKeepAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	if !result.SubjectBox.Empty {
		t.Error("SubjectBox should be empty for fully transparent image")
	}
	if result.AlphaValid {
		t.Error("AlphaValid should be false for fully transparent image")
	}
}

func TestComputeAlphaBounds(t *testing.T) {
	img := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	box := computeAlphaBounds(img, 10)
	if box.Empty {
		t.Fatal("box should not be empty")
	}
	if box.MinX != 5 || box.MinY != 5 {
		t.Errorf("MinX/MinY = %d/%d, want 5/5", box.MinX, box.MinY)
	}
	if box.MaxX != 14 || box.MaxY != 14 {
		t.Errorf("MaxX/MaxY = %d/%d, want 14/14", box.MaxX, box.MaxY)
	}
	if box.Width != 10 || box.Height != 10 {
		t.Errorf("Width/Height = %d/%d, want 10/10", box.Width, box.Height)
	}
}

func TestComputeAlphaBounds_Empty(t *testing.T) {
	img := newSolidNRGBA(10, 10, color.NRGBA{R: 0, G: 0, B: 0, A: 0})

	box := computeAlphaBounds(img, 10)
	if !box.Empty {
		t.Error("box should be empty for fully transparent image")
	}
}

func TestHasAlpha(t *testing.T) {
	withAlpha := newSolidNRGBA(5, 5, color.NRGBA{R: 100, G: 100, B: 100, A: 128})
	if !hasAlpha(withAlpha) {
		t.Error("hasAlpha should return true for image with alpha < 255")
	}

	withoutAlpha := newSolidNRGBA(5, 5, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	if hasAlpha(withoutAlpha) {
		t.Error("hasAlpha should return false for fully opaque image")
	}
}

func TestIsFullyTransparent(t *testing.T) {
	transparent := newSolidNRGBA(5, 5, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	if !isFullyTransparent(transparent) {
		t.Error("isFullyTransparent should return true for fully transparent image")
	}

	opaque := newSolidNRGBA(5, 5, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	if isFullyTransparent(opaque) {
		t.Error("isFullyTransparent should return false for opaque image")
	}
}

func TestDetectBackgroundColor(t *testing.T) {
	img := newSolidNRGBA(10, 10, color.NRGBA{R: 200, G: 100, B: 50, A: 255})

	r, g, b := detectBackgroundColor(img)
	if r != 200 || g != 100 || b != 50 {
		t.Errorf("detectBackgroundColor = (%d, %d, %d), want (200, 100, 50)", r, g, b)
	}
}

func TestDetectBackgroundColor_MixedCorners(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	img.Set(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img.Set(9, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img.Set(0, 9, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	img.Set(9, 9, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	r, g, b := detectBackgroundColor(img)
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("detectBackgroundColor = (%d, %d, %d), want (255, 255, 255) - majority color", r, g, b)
	}
}

func TestRemoveByColor(t *testing.T) {
	img := newImageWithSubject(
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		20, 20, 10, 10,
	)

	result := removeByColor(img, 255, 255, 255, 30)

	_, _, _, cornerA := result.At(0, 0).RGBA()
	if cornerA != 0 {
		t.Errorf("corner alpha = %d, want 0 (background should be transparent)", cornerA>>8)
	}

	_, _, _, centerA := result.At(10, 10).RGBA()
	if centerA == 0 {
		t.Error("center pixel should not be transparent (subject should be kept)")
	}
}

func TestConnectedComponents(t *testing.T) {
	mask := make([][]bool, 10)
	for i := range mask {
		mask[i] = make([]bool, 10)
	}

	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			mask[y][x] = true
		}
	}

	mask[7][7] = true

	result := connectedComponents(mask, 5)

	count := 0
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if result[y][x] {
				count++
			}
		}
	}

	if count != 25 {
		t.Errorf("connectedComponents kept %d pixels, want 25 (large region only)", count)
	}
}

func TestConnectedComponents_KeepsMultipleRegions(t *testing.T) {
	mask := make([][]bool, 20)
	for i := range mask {
		mask[i] = make([]bool, 20)
	}

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			mask[y][x] = true
		}
	}
	for y := 12; y < 20; y++ {
		for x := 12; x < 20; x++ {
			mask[y][x] = true
		}
	}

	result := connectedComponents(mask, 10)

	count := 0
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if result[y][x] {
				count++
			}
		}
	}

	if count != 128 {
		t.Errorf("connectedComponents kept %d pixels, want 128 (two large regions)", count)
	}
}

func TestCleanAlphaEdges(t *testing.T) {
	img := newImageWithAlphaSubject(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, 20, 20, 10, 10)

	result := cleanAlphaEdges(img, 10, 1)

	bounds := result.Bounds()
	if bounds.Dx() != 20 || bounds.Dy() != 20 {
		t.Errorf("result dimensions = %dx%d, want 20x20", bounds.Dx(), bounds.Dy())
	}

	_, _, _, cornerA := result.At(0, 0).RGBA()
	if cornerA != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent area should remain transparent)", cornerA>>8)
	}

	_, _, _, centerA := result.At(10, 10).RGBA()
	if centerA == 0 {
		t.Error("center pixel should not be transparent after cleaning")
	}
}

func TestLocalProvider_LargeSubjectNotEroded(t *testing.T) {
	p := NewLocalProvider()
	subject := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	src := newImageWithAlphaSubject(subject, 20, 20, 12, 12)

	result, err := p.RemoveBackground(context.Background(), backgroundremoval.ImageInput{
		Image:  src,
		Width:  20,
		Height: 20,
		Mode:   backgroundremoval.ModeUseExistingAlpha,
	})
	if err != nil {
		t.Fatalf("RemoveBackground error: %v", err)
	}

	bounds := result.Image.Bounds()
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2
	r, g, b, a := result.Image.At(centerX, centerY).RGBA()
	if a == 0 {
		t.Error("center pixel should not be transparent")
	}
	if r>>8 != 128 || g>>8 != 128 || b>>8 != 128 {
		t.Errorf("center pixel color = (%d, %d, %d), want (128, 128, 128)", r>>8, g>>8, b>>8)
	}
}
