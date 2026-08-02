// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package backgroundremoval

import (
	"context"
	"errors"
	"image"
	"testing"
)

type fakeProvider struct {
	name  string
	modes []BackgroundMode
}

func (f *fakeProvider) Name() string                     { return f.name }
func (f *fakeProvider) SupportedModes() []BackgroundMode { return f.modes }
func (f *fakeProvider) RemoveBackground(ctx context.Context, input ImageInput) (*BackgroundRemovalResult, error) {
	return nil, errors.New("not implemented")
}

func fakeCaps(name string, modes []BackgroundMode) BackgroundRemovalCapabilities {
	return BackgroundRemovalCapabilities{
		ProviderName:   name,
		SupportedModes: modes,
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p1 := &fakeProvider{name: "p1", modes: []BackgroundMode{ModeKeepAlpha}}
	p2 := &fakeProvider{name: "p2", modes: []BackgroundMode{ModeRemoveBackground, ModeUseExistingAlpha}}

	if err := r.Register(p1, fakeCaps("p1", p1.modes)); err != nil {
		t.Fatalf("Register(p1) error: %v", err)
	}
	if err := r.Register(p2, fakeCaps("p2", p2.modes)); err != nil {
		t.Fatalf("Register(p2) error: %v", err)
	}

	got, err := r.Get(ModeKeepAlpha)
	if err != nil {
		t.Fatalf("Get(ModeKeepAlpha) error: %v", err)
	}
	if got.Name() != "p1" {
		t.Errorf("Get(ModeKeepAlpha) = %s, want p1", got.Name())
	}

	got, err = r.Get(ModeRemoveBackground)
	if err != nil {
		t.Fatalf("Get(ModeRemoveBackground) error: %v", err)
	}
	if got.Name() != "p2" {
		t.Errorf("Get(ModeRemoveBackground) = %s, want p2", got.Name())
	}

	got, err = r.Get(ModeUseExistingAlpha)
	if err != nil {
		t.Fatalf("Get(ModeUseExistingAlpha) error: %v", err)
	}
	if got.Name() != "p2" {
		t.Errorf("Get(ModeUseExistingAlpha) = %s, want p2", got.Name())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get(ModeKeepAlpha)
	if err == nil {
		t.Fatal("Get(ModeKeepAlpha) expected error, got nil")
	}

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != ErrCodeBackgroundRemovalUnavailable {
		t.Errorf("Code = %s, want %s", pe.Code, ErrCodeBackgroundRemovalUnavailable)
	}
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Errorf("error is not ErrProviderUnavailable")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	p1 := &fakeProvider{name: "zeta", modes: []BackgroundMode{ModeKeepAlpha}}
	p2 := &fakeProvider{name: "alpha", modes: []BackgroundMode{ModeRemoveBackground}}
	p3 := &fakeProvider{name: "mid", modes: []BackgroundMode{ModeUseExistingAlpha}}

	if err := r.Register(p1, fakeCaps("zeta", p1.modes)); err != nil {
		t.Fatalf("Register(p1) error: %v", err)
	}
	if err := r.Register(p2, fakeCaps("alpha", p2.modes)); err != nil {
		t.Fatalf("Register(p2) error: %v", err)
	}
	if err := r.Register(p3, fakeCaps("mid", p3.modes)); err != nil {
		t.Fatalf("Register(p3) error: %v", err)
	}

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List() length = %d, want 3", len(list))
	}

	expected := []string{"alpha", "mid", "zeta"}
	for i, p := range list {
		if p.Name != expected[i] {
			t.Errorf("List()[%d].Name = %s, want %s", i, p.Name, expected[i])
		}
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Fatalf("List() length = %d, want 0", len(list))
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()
	p1 := &fakeProvider{name: "p1", modes: []BackgroundMode{ModeKeepAlpha}}
	p2 := &fakeProvider{name: "p1", modes: []BackgroundMode{ModeRemoveBackground}}

	if err := r.Register(p1, fakeCaps("p1", p1.modes)); err != nil {
		t.Fatalf("Register(p1) error: %v", err)
	}
	err := r.Register(p2, fakeCaps("p1", p2.modes))
	if err == nil {
		t.Fatal("Register(p2) expected error for duplicate name, got nil")
	}

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *ProviderError: %T", err)
	}
	if pe.Code != ErrCodeProviderAlreadyRegistered {
		t.Errorf("Code = %s, want %s", pe.Code, ErrCodeProviderAlreadyRegistered)
	}

	list := r.List()
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}

	_, err = r.Get(ModeRemoveBackground)
	if err == nil {
		t.Error("Get(ModeRemoveBackground) expected error, got provider")
	}
}

func TestRegistry_Register_Nil(t *testing.T) {
	r := NewRegistry()
	err := r.Register(nil, BackgroundRemovalCapabilities{})
	if err == nil {
		t.Fatal("Register(nil) expected error, got nil")
	}

	list := r.List()
	if len(list) != 0 {
		t.Fatalf("List() length = %d, want 0", len(list))
	}
}

func TestRegistry_FirstProviderWinsForMode(t *testing.T) {
	r := NewRegistry()
	p1 := &fakeProvider{name: "p1", modes: []BackgroundMode{ModeKeepAlpha}}
	p2 := &fakeProvider{name: "p2", modes: []BackgroundMode{ModeKeepAlpha}}

	if err := r.Register(p1, fakeCaps("p1", p1.modes)); err != nil {
		t.Fatalf("Register(p1) error: %v", err)
	}
	if err := r.Register(p2, fakeCaps("p2", p2.modes)); err != nil {
		t.Fatalf("Register(p2) error: %v", err)
	}

	got, err := r.Get(ModeKeepAlpha)
	if err != nil {
		t.Fatalf("Get(ModeKeepAlpha) error: %v", err)
	}
	if got.Name() != "p1" {
		t.Errorf("Get(ModeKeepAlpha) = %s, want p1 (first registered)", got.Name())
	}
}

func TestDefaultRegistry_Singleton(t *testing.T) {
	r1 := DefaultRegistry()
	r2 := DefaultRegistry()

	if r1 != r2 {
		t.Fatal("DefaultRegistry() returned different instances")
	}
}

func TestDefaultRegistry_CanRegister(t *testing.T) {
	r := DefaultRegistry()
	p := &fakeProvider{name: "default-test", modes: []BackgroundMode{ModeKeepAlpha}}
	err := r.Register(p, fakeCaps("default-test", p.modes))
	if err != nil {
		var pe *ProviderError
		if !errors.As(err, &pe) {
			t.Fatalf("Register error is not *ProviderError: %T", err)
		}
		if pe.Code != ErrCodeProviderAlreadyRegistered {
			t.Fatalf("Register error: %v", err)
		}
	}

	got, err := r.Get(ModeKeepAlpha)
	if err != nil {
		t.Fatalf("Get(ModeKeepAlpha) error: %v", err)
	}
	if got.Name() != "default-test" {
		t.Errorf("Get(ModeKeepAlpha) = %s, want default-test", got.Name())
	}
}

func TestProviderError_Error(t *testing.T) {
	err := &ProviderError{
		Code:    ErrCodeBackgroundRemovalFailed,
		Message: "test error",
		Err:     ErrBackgroundRemovalFailed,
	}

	s := err.Error()
	if s == "" {
		t.Fatal("Error() returned empty string")
	}
}

func TestProviderError_Error_NoErr(t *testing.T) {
	err := &ProviderError{
		Code:    ErrCodeBackgroundRemovalFailed,
		Message: "test error",
	}

	s := err.Error()
	if s == "" {
		t.Fatal("Error() returned empty string")
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	err := &ProviderError{
		Code:    ErrCodeBackgroundRemovalFailed,
		Message: "test error",
		Err:     ErrBackgroundRemovalFailed,
	}

	if !errors.Is(err, ErrBackgroundRemovalFailed) {
		t.Fatal("Unwrap() does not return ErrBackgroundRemovalFailed")
	}
}

func TestProviderError_Nil(t *testing.T) {
	var err *ProviderError

	if err.Error() != "" {
		t.Errorf("nil Error() = %q, want empty", err.Error())
	}
	if err.Unwrap() != nil {
		t.Errorf("nil Unwrap() = %v, want nil", err.Unwrap())
	}
}

func TestSubjectBox_Fields(t *testing.T) {
	box := SubjectBox{
		MinX:   1,
		MinY:   2,
		MaxX:   3,
		MaxY:   4,
		Width:  3,
		Height: 3,
		Empty:  false,
	}
	if box.MinX != 1 || box.MinY != 2 || box.MaxX != 3 || box.MaxY != 4 {
		t.Errorf("SubjectBox fields incorrect: %+v", box)
	}
	if box.Width != 3 || box.Height != 3 {
		t.Errorf("SubjectBox dimensions incorrect: %dx%d", box.Width, box.Height)
	}
	if box.Empty {
		t.Error("SubjectBox.Empty should be false")
	}
}

func TestImageInput_Fields(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	input := ImageInput{
		Image:  img,
		Width:  2,
		Height: 2,
		Mode:   ModeKeepAlpha,
	}
	if input.Image == nil {
		t.Error("ImageInput.Image should not be nil")
	}
	if input.Width != 2 || input.Height != 2 {
		t.Errorf("ImageInput dimensions incorrect: %dx%d", input.Width, input.Height)
	}
	if input.Mode != ModeKeepAlpha {
		t.Errorf("ImageInput.Mode = %s, want %s", input.Mode, ModeKeepAlpha)
	}
}

func TestBackgroundMode_Constants(t *testing.T) {
	if ModeKeepAlpha != "keep_alpha" {
		t.Errorf("ModeKeepAlpha = %s, want keep_alpha", ModeKeepAlpha)
	}
	if ModeRemoveBackground != "remove_background" {
		t.Errorf("ModeRemoveBackground = %s, want remove_background", ModeRemoveBackground)
	}
	if ModeUseExistingAlpha != "use_existing_alpha" {
		t.Errorf("ModeUseExistingAlpha = %s, want use_existing_alpha", ModeUseExistingAlpha)
	}
}

func TestErrorCodes_Constants(t *testing.T) {
	if ErrCodeBackgroundRemovalUnavailable != "BACKGROUND_REMOVAL_UNAVAILABLE" {
		t.Errorf("ErrCodeBackgroundRemovalUnavailable = %s", ErrCodeBackgroundRemovalUnavailable)
	}
	if ErrCodeBackgroundRemovalFailed != "BACKGROUND_REMOVAL_FAILED" {
		t.Errorf("ErrCodeBackgroundRemovalFailed = %s", ErrCodeBackgroundRemovalFailed)
	}
	if ErrCodeAlphaChannelInvalid != "ALPHA_CHANNEL_INVALID" {
		t.Errorf("ErrCodeAlphaChannelInvalid = %s", ErrCodeAlphaChannelInvalid)
	}
	if ErrCodeSubjectNotFound != "SUBJECT_NOT_FOUND" {
		t.Errorf("ErrCodeSubjectNotFound = %s", ErrCodeSubjectNotFound)
	}
}
