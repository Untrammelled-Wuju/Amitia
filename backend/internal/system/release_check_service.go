// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (s *service) GetReleaseCheckLatest() map[string]interface{} {
	versionInfo := s.GetVersion()
	version, _ := versionInfo["version"].(string)
	return map[string]interface{}{"latest": map[string]interface{}{"version": version, "date": time.Now().Format("2006-01-02"), "source": "local_release_metadata"}}
}

func (s *service) GetReleaseCheckHistory() map[string]interface{} {
	lastCheck := s.getAppSetting("last_release_check")
	versionInfo := s.GetVersion()
	version, _ := versionInfo["version"].(string)
	return map[string]interface{}{"history": []interface{}{map[string]interface{}{"version": version, "checkedAt": lastCheck, "hasUpdate": false, "source": "local_release_metadata"}}}
}

func (s *service) ExportReleaseCheck() map[string]interface{} {
	data, _ := json.Marshal(s.GetReleaseCheckHistory())
	name := fmt.Sprintf("release_check_%s.json", time.Now().Format("20060102_150405"))
	os.WriteFile(filepath.Join(s.dataDir, name), data, 0644)
	return map[string]interface{}{"exported": true, "file": name}
}

func (s *service) RunReleaseCheck() map[string]interface{} {
	s.setAppSetting("last_release_check", time.Now().Format(time.DateTime))
	return s.GetReleaseCheckLatest()
}

func (s *service) GetUpdateConfig() map[string]interface{} {
	autoCheck := s.getAppSetting("auto_update") != "false"
	return map[string]interface{}{"autoCheck": autoCheck, "channel": "stable", "lastCheckAt": nil}
}

func (s *service) UpdateUpdateConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["autoCheck"].(bool); ok {
		if v {
			s.setAppSetting("auto_update", "true")
		} else {
			s.setAppSetting("auto_update", "false")
		}
	}
	return s.GetUpdateConfig()
}
