package display

import (
	"testing"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

func TestClassifier_Default(t *testing.T) {
	c := NewDisplayClassifier()
	info := DisplayInfo{DisplayID: 0, IsDefault: true}
	got := c.Classify(info)
	if got != DisplayTypeDefault {
		t.Errorf("expected default, got %s", got)
	}
}

func TestClassifier_DefaultByID(t *testing.T) {
	c := NewDisplayClassifier()
	info := DisplayInfo{DisplayID: 0}
	got := c.Classify(info)
	if got != DisplayTypeDefault {
		t.Errorf("expected default, got %s", got)
	}
}

func TestClassifier_ManagedVirtual(t *testing.T) {
	c := NewDisplayClassifier()
	ref := virtualdisplay.VirtualDisplayRef("vd_amitia_001")
	c.SetManaged(4, &ref)
	info := DisplayInfo{DisplayID: 4}
	got := c.Classify(info)
	if got != DisplayTypeVirtualAmitia {
		t.Errorf("expected virtual_amitia, got %s", got)
	}
}

func TestClassifier_Presentation(t *testing.T) {
	c := NewDisplayClassifier()
	info := DisplayInfo{DisplayID: 2, Flags: DisplayFlags{Presentation: true}, Presentation: true}
	got := c.Classify(info)
	if got != DisplayTypePresentation {
		t.Errorf("expected presentation, got %s", got)
	}

	info2 := DisplayInfo{DisplayID: 5, Presentation: true}
	got2 := c.Classify(info2)
	if got2 != DisplayTypePresentation {
		t.Errorf("expected presentation for flag-only, got %s", got2)
	}
}

func TestClassifier_ExistingType(t *testing.T) {
	c := NewDisplayClassifier()
	info := DisplayInfo{DisplayID: 1, Type: "external"}
	got := c.Classify(info)
	if got != DisplayTypeExternal {
		t.Errorf("expected external, got %s", got)
	}
}

func TestClassifier_Unknown(t *testing.T) {
	c := NewDisplayClassifier()
	info := DisplayInfo{DisplayID: 3}
	got := c.Classify(info)
	if got != DisplayTypeUnknown {
		t.Errorf("expected unknown, got %s", got)
	}
}

func TestClassifier_RemoveManaged(t *testing.T) {
	c := NewDisplayClassifier()
	ref := virtualdisplay.VirtualDisplayRef("vd_amitia_002")
	c.SetManaged(7, &ref)
	c.RemoveManaged(7)
	info := DisplayInfo{DisplayID: 7}
	got := c.Classify(info)
	if got == DisplayTypeVirtualAmitia {
		t.Error("expected non-virtual type after removing managed ref")
	}
}
