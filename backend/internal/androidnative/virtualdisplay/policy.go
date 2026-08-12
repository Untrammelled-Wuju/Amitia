package virtualdisplay

const (
	MinWidth         = 320
	MinHeight        = 320
	MaxWidth         = 2560
	MaxHeight        = 2560
	MaxPixels        = 7372800
	MinDensityDPI    = 72
	MaxDensityDPI    = 640
	DefaultWidth     = 1080
	DefaultHeight    = 1920
	DefaultDensityDPI = 420
)

type Policy struct {
	MinWidth       int
	MinHeight      int
	MaxWidth       int
	MaxHeight      int
	MaxPixels      int
	MinDensityDPI  int
	MaxDensityDPI  int
	DefaultWidth   int
	DefaultHeight  int
	DefaultDensityDPI int
}

func DefaultPolicy() Policy {
	return Policy{
		MinWidth:       MinWidth,
		MinHeight:      MinHeight,
		MaxWidth:       MaxWidth,
		MaxHeight:      MaxHeight,
		MaxPixels:      MaxPixels,
		MinDensityDPI:  MinDensityDPI,
		MaxDensityDPI:  MaxDensityDPI,
		DefaultWidth:   DefaultWidth,
		DefaultHeight:  DefaultHeight,
		DefaultDensityDPI: DefaultDensityDPI,
	}
}

func (p *Policy) ValidateSize(width, height int) error {
	if width < p.MinWidth || width > p.MaxWidth {
		return NewError(ErrVirtualDisplayProperty, "width out of range")
	}
	if height < p.MinHeight || height > p.MaxHeight {
		return NewError(ErrVirtualDisplayProperty, "height out of range")
	}
	if int64(width)*int64(height) > int64(p.MaxPixels) {
		return NewError(ErrVirtualDisplayProperty, "pixel count exceeds maximum")
	}
	return nil
}

func (p *Policy) ValidateDensity(dpi int) error {
	if dpi < p.MinDensityDPI || dpi > p.MaxDensityDPI {
		return NewError(ErrVirtualDisplayProperty, "density out of range")
	}
	return nil
}

func (p *Policy) ValidateRefreshRate(rate float64) error {
	if rate < 0 || rate > 1000 {
		return NewError(ErrVirtualDisplayProperty, "refresh rate out of range")
	}
	return nil
}

func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func (p *Policy) ClampSize(width, height int) (int, int) {
	return clamp(width, p.MinWidth, p.MaxWidth), clamp(height, p.MinHeight, p.MaxHeight)
}

func (p *Policy) ClampDensity(dpi int) int {
	return clamp(dpi, p.MinDensityDPI, p.MaxDensityDPI)
}
