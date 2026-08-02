// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import "testing"

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"", false},
		{"0.0.0.0", false},
		{"::", false},
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"127.0.0.1:8080", true},
		{"[::1]:8080", true},
		{"localhost:8080", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"8.8.8.8", false},
	}

	for _, tt := range tests {
		got := isLoopback(tt.addr)
		if got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestSecurityConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SecurityConfig
		wantErr bool
	}{
		{
			name: "network mode empty secret",
			cfg: SecurityConfig{
				Mode:          SecurityModeNetwork,
				JWTSecret:     "",
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: true,
		},
		{
			name: "network mode with secret",
			cfg: SecurityConfig{
				Mode:          SecurityModeNetwork,
				JWTSecret:     "test-secret-key-that-is-long-enough-for-security",
				ListenAddress: "0.0.0.0:8080",
			},
			wantErr: false,
		},
		{
			name: "local_single_user no token",
			cfg: SecurityConfig{
				Mode:          SecurityModeLocalSingle,
				LocalToken:    "",
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: true,
		},
		{
			name: "local_single_user remote access",
			cfg: SecurityConfig{
				Mode:              SecurityModeLocalSingle,
				LocalToken:        "valid-token",
				AllowRemoteAccess: true,
				ListenAddress:     "127.0.0.1:8080",
			},
			wantErr: true,
		},
		{
			name: "local_single_user non-loopback",
			cfg: SecurityConfig{
				Mode:          SecurityModeLocalSingle,
				LocalToken:    "valid-token",
				ListenAddress: "192.168.1.1:8080",
			},
			wantErr: true,
		},
		{
			name: "local_single_user valid",
			cfg: SecurityConfig{
				Mode:          SecurityModeLocalSingle,
				LocalToken:    "valid-token",
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: false,
		},
		{
			name: "maintenance no credentials",
			cfg: SecurityConfig{
				Mode:          SecurityModeMaintenance,
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: true,
		},
		{
			name: "maintenance with jwt",
			cfg: SecurityConfig{
				Mode:          SecurityModeMaintenance,
				JWTSecret:     "test-secret",
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: false,
		},
		{
			name: "maintenance with local token",
			cfg: SecurityConfig{
				Mode:          SecurityModeMaintenance,
				LocalToken:    "valid-token",
				ListenAddress: "127.0.0.1:8080",
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			cfg: SecurityConfig{
				Mode: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
