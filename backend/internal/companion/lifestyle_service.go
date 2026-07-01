// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

func (s *service) GetLifestyleTendency(characterID string) map[string]interface{} {
	var t LifestyleTendency
	s.db.Table("lifestyle_tendencies").Where("character_id = ?", characterID).Limit(1).Find(&t)
	if t.ID == 0 {
		return map[string]interface{}{"punctualityTendency": 50, "earlyPrepareTendency": 50, "selfDisciplineTendency": 50, "sleepinessTendency": 50, "randomnessTendency": 50, "activityEnergy": 50, "socialEnergy": 50, "careTendency": 50, "dailyShareTendency": 50, "manuallyConfigured": false}
	}
	return map[string]interface{}{"id": t.ID, "punctualityTendency": t.PunctualityTendency, "earlyPrepareTendency": t.EarlyPrepareTendency, "selfDisciplineTendency": t.SelfDisciplineTendency, "sleepinessTendency": t.SleepinessTendency, "randomnessTendency": t.RandomnessTendency, "activityEnergy": t.ActivityEnergy, "socialEnergy": t.SocialEnergy, "careTendency": t.CareTendency, "dailyShareTendency": t.DailyShareTendency, "manuallyConfigured": t.ManuallyConfigured == 1}
}

func (s *service) UpdateLifestyleTendency(body map[string]interface{}, characterID string) map[string]interface{} {
	var count int64
	s.db.Model(&LifestyleTendency{}).Where("character_id = ?", characterID).Count(&count)
	if count == 0 {
		s.db.Create(&LifestyleTendency{CharacterID: characterID})
	}
	updates := make(map[string]interface{})
	result := map[string]interface{}{"punctualityTendency": 50, "earlyPrepareTendency": 50, "selfDisciplineTendency": 50, "sleepinessTendency": 50, "randomnessTendency": 50, "activityEnergy": 50, "socialEnergy": 50, "careTendency": 50, "dailyShareTendency": 50, "manuallyConfigured": false}
	if v, ok := body["punctualityTendency"].(float64); ok {
		updates["punctuality_tendency"] = int(v)
		result["punctualityTendency"] = int(v)
	}
	if v, ok := body["earlyPrepareTendency"].(float64); ok {
		updates["early_prepare_tendency"] = int(v)
		result["earlyPrepareTendency"] = int(v)
	}
	if v, ok := body["selfDisciplineTendency"].(float64); ok {
		updates["self_discipline_tendency"] = int(v)
		result["selfDisciplineTendency"] = int(v)
	}
	if v, ok := body["sleepinessTendency"].(float64); ok {
		updates["sleepiness_tendency"] = int(v)
		result["sleepinessTendency"] = int(v)
	}
	if v, ok := body["randomnessTendency"].(float64); ok {
		updates["randomness_tendency"] = int(v)
		result["randomnessTendency"] = int(v)
	}
	if v, ok := body["activityEnergy"].(float64); ok {
		updates["activity_energy"] = int(v)
		result["activityEnergy"] = int(v)
	}
	if v, ok := body["socialEnergy"].(float64); ok {
		updates["social_energy"] = int(v)
		result["socialEnergy"] = int(v)
	}
	if v, ok := body["careTendency"].(float64); ok {
		updates["care_tendency"] = int(v)
		result["careTendency"] = int(v)
	}
	if v, ok := body["dailyShareTendency"].(float64); ok {
		updates["daily_share_tendency"] = int(v)
		result["dailyShareTendency"] = int(v)
	}
	if v, ok := body["manuallyConfigured"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["manually_configured"] = 1
				result["manuallyConfigured"] = true
			} else {
				updates["manually_configured"] = 0
				result["manuallyConfigured"] = false
			}
		}
	}
	if len(updates) > 0 {
		s.db.Model(&LifestyleTendency{}).Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return result
}

func (s *service) ResetLifestyleTendency(characterID string) map[string]interface{} {
	s.db.Where("character_id = ?", characterID).Delete(&LifestyleTendency{})
	return s.GetLifestyleTendency(characterID)
}
