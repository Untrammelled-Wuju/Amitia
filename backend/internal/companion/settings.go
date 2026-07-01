// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

func (s *service) getSetting(key string) string {
	var v string
	s.db.Table("app_settings").Select("value").Where("key = ?", key).Row().Scan(&v)
	return v
}

func (s *service) setSetting(key, value string) {
	s.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now', 'localtime')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at", key, value)
}
