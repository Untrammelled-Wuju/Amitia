//go:build linux && !android

package archive

import "testing"

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if p.MaxEntries != 100000 {
		t.Errorf("MaxEntries = %d, want 100000", p.MaxEntries)
	}
	if p.MaxTotalUncompressedBytes != 4*1024*1024*1024 {
		t.Errorf("MaxTotalUncompressedBytes = %d, want %d", p.MaxTotalUncompressedBytes, 4*1024*1024*1024)
	}
	if p.MaxSingleEntryBytes != 1*1024*1024*1024 {
		t.Errorf("MaxSingleEntryBytes = %d, want %d", p.MaxSingleEntryBytes, 1*1024*1024*1024)
	}
	if p.MaxCompressionRatio != 1000 {
		t.Errorf("MaxCompressionRatio = %f, want 1000", p.MaxCompressionRatio)
	}
	if p.MaxArchiveBytes != 2*1024*1024*1024 {
		t.Errorf("MaxArchiveBytes = %d, want %d", p.MaxArchiveBytes, 2*1024*1024*1024)
	}
	if p.MaxSources != 10000 {
		t.Errorf("MaxSources = %d, want 10000", p.MaxSources)
	}
	if p.DefaultListLimit != 100 {
		t.Errorf("DefaultListLimit = %d, want 100", p.DefaultListLimit)
	}
	if p.MaxListLimit != 1000 {
		t.Errorf("MaxListLimit = %d, want 1000", p.MaxListLimit)
	}
	if !p.StripSetuid {
		t.Error("StripSetuid = false, want true")
	}
	if !p.StripSetgid {
		t.Error("StripSetgid = false, want true")
	}
}
