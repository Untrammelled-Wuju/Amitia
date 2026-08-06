// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestNewTarget(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	if target.Host != "127.0.0.1" {
		t.Errorf("Host = %q", target.Host)
	}
	if target.Port != 6333 {
		t.Errorf("Port = %d", target.Port)
	}
	if target.BaseURL != "http://127.0.0.1:6333" {
		t.Errorf("BaseURL = %q", target.BaseURL)
	}
	if target.Timeout != DefaultProbeTimeout {
		t.Errorf("Timeout = %v", target.Timeout)
	}
}

func TestNewTargetFromURL(t *testing.T) {
	target := NewTargetFromURL("http://localhost:6333")
	if target.BaseURL != "http://localhost:6333" {
		t.Errorf("BaseURL = %q", target.BaseURL)
	}
}

func TestTargetProbes(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)

	identity := target.IdentityProbe()
	if identity.Path != EndpointPathRoot {
		t.Errorf("IdentityProbe path = %q", identity.Path)
	}
	if identity.Timeout != DefaultIdentityTimeout {
		t.Errorf("IdentityProbe timeout = %v", identity.Timeout)
	}

	live := target.LiveProbe()
	if live.Path != EndpointPathLivez {
		t.Errorf("LiveProbe path = %q", live.Path)
	}

	ready := target.ReadyProbe()
	if ready.Path != EndpointPathReadyz {
		t.Errorf("ReadyProbe path = %q", ready.Path)
	}

	health := target.HealthProbe()
	if health.Path != EndpointPathHealthz {
		t.Errorf("HealthProbe path = %q", health.Path)
	}
}

func TestTargetWithTimeout(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	target = target.WithTimeout(10 * time.Second)
	if target.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v", target.Timeout)
	}
}

func TestTargetValidate(t *testing.T) {
	valid := NewTarget("127.0.0.1", 6333)
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate returned error: %v", err)
	}

	invalid := Target{}
	if err := invalid.Validate(); err != ErrTargetAddressRequired {
		t.Errorf("Validate should return ErrTargetAddressRequired, got %v", err)
	}
}
