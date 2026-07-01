// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

func (s *service) SetupStatus() map[string]interface{} {
	completed := s.getAppSetting("setup_completed") == "true"
	step := s.getAppSetting("setup_step")
	return map[string]interface{}{"completed": completed, "currentStep": step, "steps": []interface{}{}}
}

func (s *service) SetupChecks() map[string]interface{} {
	checks := []interface{}{}
	sqlDB, _ := s.db.DB()
	dbOk := sqlDB != nil && sqlDB.Ping() == nil
	checks = append(checks, map[string]interface{}{"name": "Database", "pass": dbOk})
	return map[string]interface{}{"checks": checks}
}

func (s *service) SetupFinish() map[string]interface{} {
	s.setAppSetting("setup_completed", "true")
	s.setAppSetting("setup_step", "done")
	return map[string]interface{}{"finished": true}
}

func (s *service) SetupReset() map[string]interface{} {
	s.setAppSetting("setup_completed", "false")
	s.setAppSetting("setup_step", "")
	return map[string]interface{}{"reset": true}
}

func (s *service) SetupStep(step string) map[string]interface{} {
	s.setAppSetting("setup_step", step)
	return map[string]interface{}{"currentStep": step, "done": false}
}

func (s *service) OnboardingStatus() map[string]interface{} {
	completed := s.getAppSetting("onboarding_completed") == "true"
	return map[string]interface{}{"completed": completed, "steps": []interface{}{}}
}

func (s *service) OnboardingComplete() map[string]interface{} {
	s.setAppSetting("onboarding_completed", "true")
	return map[string]interface{}{"completed": true}
}

func (s *service) OnboardingReset() map[string]interface{} {
	s.setAppSetting("onboarding_completed", "false")
	return map[string]interface{}{"reset": true}
}
