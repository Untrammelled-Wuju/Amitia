package virtualdisplay

import "testing"

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.MinWidth != 320 {
		t.Errorf("MinWidth: got %d, want 320", p.MinWidth)
	}
	if p.MaxWidth != 2560 {
		t.Errorf("MaxWidth: got %d, want 2560", p.MaxWidth)
	}
	if p.DefaultWidth != 1080 {
		t.Errorf("DefaultWidth: got %d, want 1080", p.DefaultWidth)
	}
	if p.DefaultHeight != 1920 {
		t.Errorf("DefaultHeight: got %d, want 1920", p.DefaultHeight)
	}
	if p.DefaultDensityDPI != 420 {
		t.Errorf("DefaultDensityDPI: got %d, want 420", p.DefaultDensityDPI)
	}
}

func TestPolicy_ValidateSize(t *testing.T) {
	p := DefaultPolicy()
	cases := []struct {
		width, height int
		wantErr       bool
	}{
		{320, 320, false},
		{1080, 1920, false},
		{2560, 2560, false},
		{100, 100, true},
		{3000, 3000, true},
		{320, 3000, true},
		{3000, 320, true},
	}
	for _, tc := range cases {
		err := p.ValidateSize(tc.width, tc.height)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateSize(%d,%d): expected error", tc.width, tc.height)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateSize(%d,%d): unexpected error: %v", tc.width, tc.height, err)
		}
	}
}

func TestPolicy_ValidateSize_Pixels(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateSize(3000, 3000)
	if err == nil {
		t.Error("expected error for excessive pixels")
	}
}

func TestPolicy_ValidateDensity(t *testing.T) {
	p := DefaultPolicy()
	cases := []struct {
		dpi     int
		wantErr bool
	}{
		{72, false},
		{420, false},
		{640, false},
		{50, true},
		{700, true},
	}
	for _, tc := range cases {
		err := p.ValidateDensity(tc.dpi)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateDensity(%d): expected error", tc.dpi)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateDensity(%d): unexpected error: %v", tc.dpi, err)
		}
	}
}

func TestPolicy_ValidateRefreshRate(t *testing.T) {
	p := DefaultPolicy()
	cases := []struct {
		rate    float64
		wantErr bool
	}{
		{0, false},
		{60, false},
		{120, false},
		{-1, true},
		{1001, true},
	}
	for _, tc := range cases {
		err := p.ValidateRefreshRate(tc.rate)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateRefreshRate(%f): expected error", tc.rate)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateRefreshRate(%f): unexpected error: %v", tc.rate, err)
		}
	}
}

func TestPolicy_ClampSize(t *testing.T) {
	p := DefaultPolicy()
	w, h := p.ClampSize(100, 100)
	if w != 320 || h != 320 {
		t.Errorf("ClampSize(100,100) = (%d,%d), want (320,320)", w, h)
	}
	w, h = p.ClampSize(3000, 3000)
	if w != 2560 || h != 2560 {
		t.Errorf("ClampSize(3000,3000) = (%d,%d), want (2560,2560)", w, h)
	}
}

func TestPolicy_ClampDensity(t *testing.T) {
	p := DefaultPolicy()
	cases := []struct {
		dpi, want int
	}{
		{50, 72},
		{420, 420},
		{700, 640},
	}
	for _, tc := range cases {
		got := p.ClampDensity(tc.dpi)
		if got != tc.want {
			t.Errorf("ClampDensity(%d) = %d, want %d", tc.dpi, got, tc.want)
		}
	}
}
