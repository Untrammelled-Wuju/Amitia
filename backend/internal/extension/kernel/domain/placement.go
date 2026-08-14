package domain

func (p ExtensionPlacement) IsValid() bool {
	return IsKnownExtensionPlacement(p)
}

func (p ExtensionPlacement) String() string {
	return string(p)
}

func (p ModulePlacement) IsValid() bool {
	return IsKnownModulePlacement(p)
}

func (p ModulePlacement) String() string {
	return string(p)
}

func (r DeviceRequirements) IsZero() bool {
	return len(r.Platforms) == 0 &&
		len(r.Architectures) == 0 &&
		r.MinAppVersion == "" &&
		r.MinRuntimeVersion == "" &&
		len(r.RequiredFeatures) == 0
}
