// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

func (s *service) getAppSetting(key string) string {
	var val string
	s.db.Table("app_settings").Select("value").Where("key = ?", key).Row().Scan(&val)
	return val
}

func (s *service) setAppSetting(key, val string) {
	result := s.db.Table("app_settings").Where("key = ?", key).Update("value", val)
	if result.RowsAffected == 0 {
		s.db.Table("app_settings").Create(map[string]interface{}{"key": key, "value": val})
	}
}
