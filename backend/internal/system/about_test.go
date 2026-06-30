// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"testing"
)

func TestGetAbout(t *testing.T) {
	t.Setenv("AMITIA_VERSION", "2.3.4")
	t.Setenv("AMITIA_GIT_COMMIT", "abc123")
	t.Setenv("AMITIA_SOURCE_CODE_URL", "https://example.com/source")
	t.Setenv("AMITIA_COMMERCIAL_LICENSE_URL", "https://example.com/commercial")
	t.Setenv("AMITIA_THIRD_PARTY_NOTICES_URL", "https://example.com/notices")

	svc := &service{}
	about := svc.GetAbout()

	if about["name"] != "Amitia" {
		t.Fatalf("name = %v", about["name"])
	}
	if about["displayName"] != "阿米提亚" {
		t.Fatalf("displayName = %v", about["displayName"])
	}
	if about["license"] != "AGPL-3.0-only" {
		t.Fatalf("license = %v", about["license"])
	}
	if about["copyright"] != "Copyright (C) 2026 彭旭" {
		t.Fatalf("copyright = %v", about["copyright"])
	}
	if about["version"] != "2.3.4" {
		t.Fatalf("version = %v", about["version"])
	}
	if about["gitCommit"] != "abc123" {
		t.Fatalf("gitCommit = %v", about["gitCommit"])
	}
	if about["sourceCodeUrl"] != "https://example.com/source" {
		t.Fatalf("sourceCodeUrl = %v", about["sourceCodeUrl"])
	}
	if about["commercialLicensingUrl"] != "https://example.com/commercial" {
		t.Fatalf("commercialLicensingUrl = %v", about["commercialLicensingUrl"])
	}
	if about["thirdPartyNoticesUrl"] != "https://example.com/notices" {
		t.Fatalf("thirdPartyNoticesUrl = %v", about["thirdPartyNoticesUrl"])
	}
}

func TestGetAboutDefaults(t *testing.T) {
	svc := &service{}
	about := svc.GetAbout()

	if about["sourceCodeUrl"] != "https://gitee.com/Untrammelled/Amitia" {
		t.Fatalf("sourceCodeUrl = %v", about["sourceCodeUrl"])
	}
	if about["commercialLicensingUrl"] != "mailto:3151508592@qq.com" {
		t.Fatalf("commercialLicensingUrl = %v", about["commercialLicensingUrl"])
	}
	if about["thirdPartyNoticesUrl"] != "https://gitee.com/Untrammelled/Amitia/blob/master/THIRD_PARTY_NOTICES.md" {
		t.Fatalf("thirdPartyNoticesUrl = %v", about["thirdPartyNoticesUrl"])
	}
}

func TestGetVersion(t *testing.T) {
	t.Setenv("AMITIA_VERSION", "9.8.7")
	t.Setenv("AMITIA_BUILD_TIME", "2026-06-30")

	svc := &service{}
	version := svc.GetVersion()

	if version["version"] != "9.8.7" {
		t.Fatalf("version = %v", version["version"])
	}
	if version["buildTime"] != "2026-06-30" {
		t.Fatalf("buildTime = %v", version["buildTime"])
	}
	if version["goVersion"] == "" {
		t.Fatal("goVersion is empty")
	}
}
