package resource

import "testing"

func TestSubjectMapper_Validate_OK(t *testing.T) {
	reader := newFakeReader()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	mapper := NewSubjectMapper(reader)
	subj, err := mapper.Validate("rt-1", "p-1", "s-1", 1)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if subj.ExtensionID != "ext-1" {
		t.Fatalf("expected ext-1, got %s", subj.ExtensionID)
	}
}

func TestSubjectMapper_Validate_EmptyRuntimeID(t *testing.T) {
	mapper := NewSubjectMapper(newFakeReader())
	_, err := mapper.Validate("", "p", "s", 1)
	if err != ErrSubjectInvalid {
		t.Fatalf("expected ErrSubjectInvalid, got %v", err)
	}
}

func TestSubjectMapper_Validate_PluginMismatch(t *testing.T) {
	reader := newFakeReader()
	reader.AddRuntime("rt-1", "p-actual", "ext-1")
	reader.AddService("rt-1", "s-1", "p-actual", "ext-1")

	mapper := NewSubjectMapper(reader)
	_, err := mapper.Validate("rt-1", "p-other", "s-1", 1)
	if err != ErrSubjectInvalid {
		t.Fatalf("expected ErrSubjectInvalid, got %v", err)
	}
}

func TestSubjectMapper_Validate_RuntimeNotExist(t *testing.T) {
	mapper := NewSubjectMapper(newFakeReader())
	_, err := mapper.Validate("missing", "p", "s", 1)
	if err != ErrRuntimeNotFound {
		t.Fatalf("expected ErrRuntimeNotFound, got %v", err)
	}
}

func TestSubjectMapper_Validate_ServiceNotExist(t *testing.T) {
	reader := newFakeReader()
	reader.AddRuntime("rt-1", "p-1", "ext-1")

	mapper := NewSubjectMapper(reader)
	_, err := mapper.Validate("rt-1", "p-1", "missing", 1)
	if err != ErrServiceNotFound {
		t.Fatalf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestSubjectMapper_Validate_ExtensionDisabled(t *testing.T) {
	reader := newFakeReader()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")
	reader.SetDisabled("ext-1", true)

	mapper := NewSubjectMapper(reader)
	_, err := mapper.Validate("rt-1", "p-1", "s-1", 1)
	if err != ErrExtensionDisabled {
		t.Fatalf("expected ErrExtensionDisabled, got %v", err)
	}
}

func TestSubjectMapper_Validate_NilReaderFailsClosed(t *testing.T) {
	mapper := NewSubjectMapper(nil)
	_, err := mapper.Validate("rt-1", "p-1", "s-1", 1)
	if err != ErrSubjectInvalid {
		t.Fatalf("nil reader must fail closed with ErrSubjectInvalid, got %v", err)
	}
}

func TestSubjectMapper_Validate_GenerationMismatch(t *testing.T) {
	reader := newFakeReader()
	reader.AddRuntime("rt-1", "p-1", "ext-1")
	reader.AddService("rt-1", "s-1", "p-1", "ext-1")

	mapper := NewSubjectMapper(reader)
	if _, err := mapper.Validate("rt-1", "p-1", "s-1", 2); err == nil {
		t.Fatal("stale or future generation must be rejected")
	}
}
