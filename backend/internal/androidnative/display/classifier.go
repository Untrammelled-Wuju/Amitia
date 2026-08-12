package display

import "github.com/u-ai/backend/internal/androidnative/virtualdisplay"

type DisplayClassifier struct {
	managed map[int]*virtualdisplay.VirtualDisplayRef
}

func NewDisplayClassifier() *DisplayClassifier {
	return &DisplayClassifier{
		managed: make(map[int]*virtualdisplay.VirtualDisplayRef),
	}
}

func (c *DisplayClassifier) SetManaged(displayID int, ref *virtualdisplay.VirtualDisplayRef) {
	c.managed[displayID] = ref
}

func (c *DisplayClassifier) RemoveManaged(displayID int) {
	delete(c.managed, displayID)
}

func (c *DisplayClassifier) IsManaged(displayID int) (*virtualdisplay.VirtualDisplayRef, bool) {
	ref, ok := c.managed[displayID]
	return ref, ok
}

func (c *DisplayClassifier) Classify(info DisplayInfo) DisplayType {
	if ref, ok := c.IsManaged(info.DisplayID); ok && ref != nil {
		return DisplayTypeVirtualAmitia
	}

	if info.DisplayID == DefaultDisplayID || info.IsDefault {
		return DisplayTypeDefault
	}

	flags := info.Flags

	if flags.Presentation || info.Presentation {
		return DisplayTypePresentation
	}

	if flags.Private && !info.IsDefault {
		return DisplayTypeUnknown
	}

	if info.Type != "" {
		switch DisplayType(info.Type) {
		case DisplayTypeBuiltIn, DisplayTypeExternal, DisplayTypeWireless,
			DisplayTypePresentation, DisplayTypeVirtualExternal, DisplayTypeDefault:
			return DisplayType(info.Type)
		}
	}

	return DisplayTypeUnknown
}
