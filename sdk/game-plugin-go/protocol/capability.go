package protocol

import "fmt"

func ValidateHostFeature(feature HostFeature) error {
	if !IsKnownHostFeature(feature) {
		return fmt.Errorf("unknown host feature %q", feature)
	}
	return nil
}

func ValidateHostFeatures(features []HostFeature) error {
	seen := make(map[HostFeature]bool, len(features))
	for i := range features {
		if err := ValidateHostFeature(features[i]); err != nil {
			return fmt.Errorf("hostFeatures[%d]: %w", i, err)
		}
		if seen[features[i]] {
			return fmt.Errorf("duplicate host feature %q", features[i])
		}
		seen[features[i]] = true
	}
	return nil
}
