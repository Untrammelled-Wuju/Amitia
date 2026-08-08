package domain

import "testing"

func TestServiceDescriptorValid(t *testing.T) {
	cases := []ServiceDescriptor{
		{
			ID:   ServiceID("svc-1"),
			Name: "Service 1",
			Kind: ServiceKindProcess,
		},
		{
			ID:   ServiceID("svc-2"),
			Name: "Service 2",
			Kind: ServiceKindExternal,
		},
		{
			ID:   ServiceID("svc-3"),
			Name: "Service 3",
			Kind: ServiceKindProcess,
			DependsOn: []ServiceID{
				ServiceID("svc-1"),
				ServiceID("svc-2"),
			},
		},
	}

	for i, svc := range cases {
		t.Run("valid_"+string(svc.ID), func(t *testing.T) {
			if err := svc.Validate(); err != nil {
				t.Errorf("case %d: expected valid, got: %v", i, err)
			}
		})
	}
}

func TestServiceDescriptorRejectsEmptyID(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID(""),
		Name: "Test",
		Kind: ServiceKindProcess,
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for empty service id")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestServiceDescriptorRejectsEmptyName(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "",
		Kind: ServiceKindProcess,
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for empty service name")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestServiceDescriptorRejectsInvalidKind(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "Test",
		Kind: ServiceKind("invalid_kind"),
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for invalid service kind")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestServiceDescriptorRejectsSelfDependency(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "Test",
		Kind: ServiceKindProcess,
		DependsOn: []ServiceID{
			ServiceID("svc-1"),
		},
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestServiceDescriptorRejectsEmptyDependencyID(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "Test",
		Kind: ServiceKindProcess,
		DependsOn: []ServiceID{
			ServiceID(""),
		},
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for empty dependency id")
	}
}

func TestServiceDescriptorRejectsControlCharactersInID(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("bad\x00id"),
		Name: "Test",
		Kind: ServiceKindProcess,
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for control character in id")
	}
}

func TestServiceDescriptorRejectsControlCharactersInName(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "bad\x00name",
		Kind: ServiceKindProcess,
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for control character in name")
	}
}

func TestServiceDescriptorWithMetadata(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "Test",
		Kind: ServiceKindProcess,
		Metadata: map[string]string{
			"host": "node-01",
			"port": "8080",
		},
	}

	if err := svc.Validate(); err != nil {
		t.Fatalf("expected metadata to be valid, got: %v", err)
	}
}

func TestServiceDescriptorRejectsInvalidMetadata(t *testing.T) {
	svc := ServiceDescriptor{
		ID:   ServiceID("svc-1"),
		Name: "Test",
		Kind: ServiceKindProcess,
		Metadata: map[string]string{
			"": "empty key",
		},
	}

	err := svc.Validate()
	if err == nil {
		t.Fatal("expected error for empty metadata key")
	}
}

func TestIsValidServiceKind(t *testing.T) {
	if !IsValidServiceKind(ServiceKindProcess) {
		t.Error("expected process to be valid")
	}
	if !IsValidServiceKind(ServiceKindExternal) {
		t.Error("expected external to be valid")
	}
	if IsValidServiceKind(ServiceKind("invalid")) {
		t.Error("expected invalid kind to be rejected")
	}
}
