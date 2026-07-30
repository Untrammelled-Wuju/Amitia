// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func setupTwoInstallations(t *testing.T) (Service, *gorm.DB, string, *Installation, *Installation, *mockNotifier) {
	t.Helper()
	db := setupTestDB(t)
	dataDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}

	pkgRepo, charRepo := newDefaultStubRepos()

	pkgA := createReadyPackage(t, dataDir, "pkg_a", defaultTestActions())
	pkgRepo.pkgs["pkg_a"] = pkgA
	pkgB := createReadyPackage(t, dataDir, "pkg_b", defaultTestActions())
	pkgRepo.pkgs["pkg_b"] = pkgB

	svc := newTestService(t, db, dataDir, pkgRepo, charRepo)
	notifier := &mockNotifier{}
	SetRuntimeNotifier(svc, notifier)

	instA, err := svc.InstallPackage("pkg_a", testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage A: %v", err)
	}
	instB, err := svc.InstallPackage("pkg_b", testUserID, testCharacterID)
	if err != nil {
		t.Fatalf("InstallPackage B: %v", err)
	}
	return svc, db, dataDir, instA, instB, notifier
}

func TestEnable_Success(t *testing.T) {
	svc, db, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusEnabled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusEnabled)
	}
	if dbInst.IsActive != 1 {
		t.Fatalf("IsActive = %d, 期望 1", dbInst.IsActive)
	}
	if len(notifier.enabledCalls) != 1 {
		t.Fatalf("期望 1 次 enabled 通知, 实际 %d", len(notifier.enabledCalls))
	}
	if notifier.enabledCalls[0].InstallationID != inst.ID {
		t.Fatalf("通知 InstallationID = %s, 期望 %s", notifier.enabledCalls[0].InstallationID, inst.ID)
	}
}

func TestEnable_CreatesRuntimeSettings(t *testing.T) {
	svc, db, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	rs := getRuntimeSettingsFromDB(t, db, inst.ID)
	if rs.Scale != runtimeSettingsDefaultScale {
		t.Fatalf("Scale = %v, 期望 %v", rs.Scale, runtimeSettingsDefaultScale)
	}
	if rs.IdleIntervalMinSeconds != runtimeSettingsDefaultIdleIntervalMinSeconds {
		t.Fatalf("IdleIntervalMinSeconds = %d, 期望 %d", rs.IdleIntervalMinSeconds, runtimeSettingsDefaultIdleIntervalMinSeconds)
	}
	if rs.IdleIntervalMaxSeconds != runtimeSettingsDefaultIdleIntervalMaxSeconds {
		t.Fatalf("IdleIntervalMaxSeconds = %d, 期望 %d", rs.IdleIntervalMaxSeconds, runtimeSettingsDefaultIdleIntervalMaxSeconds)
	}
	if rs.ClickThroughMode != runtimeSettingsDefaultClickThroughMode {
		t.Fatalf("ClickThroughMode = %s, 期望 %s", rs.ClickThroughMode, runtimeSettingsDefaultClickThroughMode)
	}
}

func TestEnable_SingleInstance_OldActiveDeactivated(t *testing.T) {
	svc, db, _, instA, instB, notifier := setupTwoInstallations(t)

	if err := svc.EnableInstallation(testUserID, instA.ID); err != nil {
		t.Fatalf("EnableInstallation A: %v", err)
	}
	if err := svc.EnableInstallation(testUserID, instB.ID); err != nil {
		t.Fatalf("EnableInstallation B: %v", err)
	}

	dbA := getInstallationFromDB(t, db, instA.ID)
	if dbA.IsActive != 0 {
		t.Fatalf("A IsActive = %d, 期望 0", dbA.IsActive)
	}

	dbB := getInstallationFromDB(t, db, instB.ID)
	if dbB.IsActive != 1 {
		t.Fatalf("B IsActive = %d, 期望 1", dbB.IsActive)
	}
	if dbB.Status != StatusEnabled {
		t.Fatalf("B 状态 = %s, 期望 %s", dbB.Status, StatusEnabled)
	}

	if len(notifier.enabledCalls) != 2 {
		t.Fatalf("期望 2 次 enabled 通知, 实际 %d", len(notifier.enabledCalls))
	}
}

func TestEnable_Idempotent(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("首次 EnableInstallation: %v", err)
	}
	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("重复 EnableInstallation 应幂等: %v", err)
	}
}

func TestEnable_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.EnableInstallation(testUserID, "nonexistent")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestEnable_NotOwnedByUser_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	err := svc.EnableInstallation("other_user", inst.ID)
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestEnable_EmptyParams_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.EnableInstallation("", "inst_id")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.EnableInstallation(testUserID, "")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestEnable_Uninstalled_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.Uninstall(testUserID, inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	err := svc.EnableInstallation(testUserID, inst.ID)
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestEnable_PackageHashCorrupt_Rejected(t *testing.T) {
	svc, db, dataDir, inst, _ := setupInstalledService(t)

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	writeFile(t, filepath.Join(installDir, "extra_file.txt"), []byte("corrupt"))

	err := svc.EnableInstallation(testUserID, inst.ID)
	assertInstallationError(t, err, ErrCodePackageHashMismatch)

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status == StatusEnabled {
		t.Fatalf("状态不应为 enabled, 实际 %s", dbInst.Status)
	}
}

func TestDisable_Success(t *testing.T) {
	svc, db, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	if err := svc.DisableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("DisableInstallation: %v", err)
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.Status != StatusDisabled {
		t.Fatalf("状态 = %s, 期望 %s", dbInst.Status, StatusDisabled)
	}
	if dbInst.IsActive != 0 {
		t.Fatalf("IsActive = %d, 期望 0", dbInst.IsActive)
	}
	if len(notifier.disabledCalls) != 1 {
		t.Fatalf("期望 1 次 disabled 通知, 实际 %d", len(notifier.disabledCalls))
	}
}

func TestDisable_Idempotent(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.DisableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("首次 DisableInstallation: %v", err)
	}
	if err := svc.DisableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("重复 DisableInstallation 应幂等: %v", err)
	}
}

func TestDisable_PreservesRuntimeSettings(t *testing.T) {
	svc, db, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	px, py := 100, 200
	sc := 1.5
	if _, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		PositionX: &px,
		PositionY: &py,
		Scale:     &sc,
	}); err != nil {
		t.Fatalf("UpdateRuntimeSettings: %v", err)
	}

	if err := svc.DisableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("DisableInstallation: %v", err)
	}

	rs := getRuntimeSettingsFromDB(t, db, inst.ID)
	if rs.PositionX != 100 {
		t.Fatalf("PositionX = %d, 期望 100", rs.PositionX)
	}
	if rs.PositionY != 200 {
		t.Fatalf("PositionY = %d, 期望 200", rs.PositionY)
	}
	if rs.Scale != 1.5 {
		t.Fatalf("Scale = %v, 期望 1.5", rs.Scale)
	}
}

func TestDisable_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.DisableInstallation(testUserID, "nonexistent")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestDisable_AlreadyUninstalled_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.Uninstall(testUserID, inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	err := svc.DisableInstallation(testUserID, inst.ID)
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestSwitch_DisablesOldAndEnablesNew(t *testing.T) {
	svc, db, _, instA, instB, notifier := setupTwoInstallations(t)

	if err := svc.EnableInstallation(testUserID, instA.ID); err != nil {
		t.Fatalf("EnableInstallation A: %v", err)
	}

	if err := svc.SwitchInstallation(testUserID, instB.ID); err != nil {
		t.Fatalf("SwitchInstallation: %v", err)
	}

	dbA := getInstallationFromDB(t, db, instA.ID)
	if dbA.IsActive != 0 {
		t.Fatalf("A IsActive = %d, 期望 0", dbA.IsActive)
	}
	if dbA.Status != StatusDisabled {
		t.Fatalf("A 状态 = %s, 期望 %s", dbA.Status, StatusDisabled)
	}

	dbB := getInstallationFromDB(t, db, instB.ID)
	if dbB.IsActive != 1 {
		t.Fatalf("B IsActive = %d, 期望 1", dbB.IsActive)
	}
	if dbB.Status != StatusEnabled {
		t.Fatalf("B 状态 = %s, 期望 %s", dbB.Status, StatusEnabled)
	}

	if len(notifier.enabledCalls) < 2 {
		t.Fatalf("期望至少 2 次 enabled 通知, 实际 %d", len(notifier.enabledCalls))
	}
	if len(notifier.disabledCalls) < 1 {
		t.Fatalf("期望至少 1 次 disabled 通知, 实际 %d", len(notifier.disabledCalls))
	}
}

func TestSwitch_SameInstance_NoOp(t *testing.T) {
	svc, _, _, instA, instB, _ := setupTwoInstallations(t)

	if err := svc.EnableInstallation(testUserID, instA.ID); err != nil {
		t.Fatalf("EnableInstallation A: %v", err)
	}

	if err := svc.SwitchInstallation(testUserID, instA.ID); err != nil {
		t.Fatalf("SwitchInstallation 同一实例应不报错: %v", err)
	}

	_ = instB
}

func TestSwitch_TargetUninstalled_Rejected(t *testing.T) {
	svc, _, _, instA, instB, _ := setupTwoInstallations(t)

	if err := svc.EnableInstallation(testUserID, instA.ID); err != nil {
		t.Fatalf("EnableInstallation A: %v", err)
	}

	if err := svc.Uninstall(testUserID, instB.ID); err != nil {
		t.Fatalf("Uninstall B: %v", err)
	}

	err := svc.SwitchInstallation(testUserID, instB.ID)
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestSwitch_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _, _ := setupTwoInstallations(t)

	err := svc.SwitchInstallation(testUserID, "nonexistent")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestUpdateDefaultAction_SupportsIdle_Success(t *testing.T) {
	svc, db, dataDir, inst, notifier := setupInstalledService(t)

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	manifestPath := filepath.Join(installDir, "manifest.json")
	manifestBefore, _ := os.ReadFile(manifestPath)

	if err := svc.UpdateDefaultAction(inst.ID, "idle_normal"); err != nil {
		t.Fatalf("UpdateDefaultAction: %v", err)
	}

	manifestAfter, _ := os.ReadFile(manifestPath)
	if string(manifestBefore) != string(manifestAfter) {
		t.Fatal("manifest.json 不应被修改")
	}

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.DefaultActionKey != "idle_normal" {
		t.Fatalf("DefaultActionKey = %s, 期望 idle_normal", dbInst.DefaultActionKey)
	}

	if len(notifier.defaultChanged) != 1 {
		t.Fatalf("期望 1 次默认动作变更通知, 实际 %d", len(notifier.defaultChanged))
	}
}

func TestUpdateDefaultAction_DoesNotSupportIdle_Rejected(t *testing.T) {
	svc, db, dataDir, inst, _ := setupInstalledService(t)

	manifestBefore := getInstallationFromDB(t, db, inst.ID).DefaultActionKey

	err := svc.UpdateDefaultAction(inst.ID, "wave")
	assertInstallationError(t, err, ErrCodeDefaultActionNotIdle)

	dbInst := getInstallationFromDB(t, db, inst.ID)
	if dbInst.DefaultActionKey != manifestBefore {
		t.Fatalf("DefaultActionKey 应不变 = %s, 实际 %s", manifestBefore, dbInst.DefaultActionKey)
	}

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	manifestPath := filepath.Join(installDir, "manifest.json")
	manifestData, _ := os.ReadFile(manifestPath)
	if !contains(manifestData, "\"defaultAction\": \"idle_normal\"") {
		t.Fatalf("manifest.json defaultAction 应仍为 idle_normal: %s", string(manifestData))
	}
}

func TestUpdateDefaultAction_ManifestNotModified(t *testing.T) {
	svc, _, dataDir, inst, _ := setupInstalledService(t)

	installDir := filepath.Join(dataDir, filepath.FromSlash(inst.InstallPath))
	manifestPath := filepath.Join(installDir, "manifest.json")
	manifestBefore, _ := os.ReadFile(manifestPath)

	_ = svc.UpdateDefaultAction(inst.ID, "wave")

	manifestAfter, _ := os.ReadFile(manifestPath)
	if string(manifestBefore) != string(manifestAfter) {
		t.Fatal("拒绝更换时 manifest.json 也不应被修改")
	}
}

func TestUpdateDefaultAction_ActionNotFound_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	err := svc.UpdateDefaultAction(inst.ID, "nonexistent_action")
	assertInstallationError(t, err, ErrCodeActionNotFound)
}

func TestUpdateDefaultAction_InstallationNotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.UpdateDefaultAction("nonexistent", "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestUpdateDefaultAction_EmptyParams_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	err := svc.UpdateDefaultAction("", "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.UpdateDefaultAction(inst.ID, "")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestUpdateDefaultAction_Uninstalled_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.Uninstall(testUserID, inst.ID); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	err := svc.UpdateDefaultAction(inst.ID, "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestPlayAction_Success_NotifiesScheduler(t *testing.T) {
	svc, _, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	if err := svc.PlayAction(testUserID, inst.ID, "idle_normal"); err != nil {
		t.Fatalf("PlayAction: %v", err)
	}

	if len(notifier.actionPlayCalls) != 1 {
		t.Fatalf("期望 1 次 actionPlay 通知, 实际 %d", len(notifier.actionPlayCalls))
	}
	if notifier.actionPlayCalls[0].ActionKey != "idle_normal" {
		t.Fatalf("通知 ActionKey = %s, 期望 idle_normal", notifier.actionPlayCalls[0].ActionKey)
	}
	if notifier.actionPlayCalls[0].InstallationID != inst.ID {
		t.Fatalf("通知 InstallationID = %s, 期望 %s", notifier.actionPlayCalls[0].InstallationID, inst.ID)
	}
}

func TestPlayAction_PlayWave_Success(t *testing.T) {
	svc, _, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	if err := svc.PlayAction(testUserID, inst.ID, "wave"); err != nil {
		t.Fatalf("PlayAction wave: %v", err)
	}

	if len(notifier.actionPlayCalls) != 1 {
		t.Fatalf("期望 1 次 actionPlay 通知, 实际 %d", len(notifier.actionPlayCalls))
	}
	if notifier.actionPlayCalls[0].ActionKey != "wave" {
		t.Fatalf("通知 ActionKey = %s, 期望 wave", notifier.actionPlayCalls[0].ActionKey)
	}
}

func TestPlayAction_PetNotEnabled_Rejected(t *testing.T) {
	svc, _, _, inst, notifier := setupInstalledService(t)

	err := svc.PlayAction(testUserID, inst.ID, "idle_normal")
	assertInstallationError(t, err, ErrCodePetNotEnabled)

	if len(notifier.actionPlayCalls) != 0 {
		t.Fatalf("不应发送 actionPlay 通知, 实际 %d", len(notifier.actionPlayCalls))
	}
}

func TestPlayAction_ActionNotFound_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	err := svc.PlayAction(testUserID, inst.ID, "nonexistent_action")
	assertInstallationError(t, err, ErrCodeActionNotFound)
}

func TestPlayAction_NotOwnedByUser_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	err := svc.PlayAction("other_user", inst.ID, "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestPlayAction_InstallationNotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.PlayAction(testUserID, "nonexistent", "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestPlayAction_EmptyParams_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	err := svc.PlayAction("", inst.ID, "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.PlayAction(testUserID, "", "idle_normal")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)

	err = svc.PlayAction(testUserID, inst.ID, "")
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestPlayAction_DoesNotTriggerGeneration(t *testing.T) {
	svc, _, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := svc.PlayAction(testUserID, inst.ID, "idle_normal"); err != nil {
			t.Fatalf("PlayAction %d: %v", i, err)
		}
	}

	if len(notifier.actionPlayCalls) != 5 {
		t.Fatalf("期望 5 次 actionPlay 通知, 实际 %d", len(notifier.actionPlayCalls))
	}
}

func TestRecenter_Success(t *testing.T) {
	svc, db, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	px, py := 500, 600
	if _, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		PositionX: &px,
		PositionY: &py,
	}); err != nil {
		t.Fatalf("UpdateRuntimeSettings: %v", err)
	}

	if err := svc.Recenter(testUserID, inst.ID); err != nil {
		t.Fatalf("Recenter: %v", err)
	}

	rs := getRuntimeSettingsFromDB(t, db, inst.ID)
	if rs.PositionMode != "recenter" {
		t.Fatalf("PositionMode = %s, 期望 recenter", rs.PositionMode)
	}
	if rs.SettingsRevision != 2 {
		t.Fatalf("SettingsRevision = %d, 期望 2", rs.SettingsRevision)
	}

	if len(notifier.recenterCalls) != 1 {
		t.Fatalf("期望 1 次 recenter 通知, 实际 %d", len(notifier.recenterCalls))
	}
}

func TestRecenter_PetNotEnabled_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	err := svc.Recenter(testUserID, inst.ID)
	assertInstallationError(t, err, ErrCodePetNotEnabled)
}

func TestRecenter_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	err := svc.Recenter(testUserID, "nonexistent")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestUpdateRuntimeSettings_Success(t *testing.T) {
	svc, db, _, inst, notifier := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	sc := 2.0
	px, py := 300, 400
	aot := 0
	ctm := "full"
	if _, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		Scale:            &sc,
		PositionX:        &px,
		PositionY:        &py,
		AlwaysOnTop:      &aot,
		ClickThroughMode: &ctm,
	}); err != nil {
		t.Fatalf("UpdateRuntimeSettings: %v", err)
	}

	rs := getRuntimeSettingsFromDB(t, db, inst.ID)
	if rs.Scale != 2.0 {
		t.Fatalf("Scale = %v, 期望 2.0", rs.Scale)
	}
	if rs.PositionX != 300 {
		t.Fatalf("PositionX = %d, 期望 300", rs.PositionX)
	}
	if rs.PositionY != 400 {
		t.Fatalf("PositionY = %d, 期望 400", rs.PositionY)
	}
	if rs.AlwaysOnTop != 0 {
		t.Fatalf("AlwaysOnTop = %d, 期望 0", rs.AlwaysOnTop)
	}
	if rs.ClickThroughMode != "full" {
		t.Fatalf("ClickThroughMode = %s, 期望 full", rs.ClickThroughMode)
	}
	if rs.SettingsRevision != 1 {
		t.Fatalf("SettingsRevision = %d, 期望 1", rs.SettingsRevision)
	}

	if len(notifier.settingsUpdated) != 1 {
		t.Fatalf("期望 1 次 settingsUpdated 通知, 实际 %d", len(notifier.settingsUpdated))
	}
}

func TestUpdateRuntimeSettings_InvalidClickThroughMode_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	ctm := "invalid_mode"
	_, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		ClickThroughMode: &ctm,
	})
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestUpdateRuntimeSettings_EmptySettings_NoOp(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	if _, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{}); err != nil {
		t.Fatalf("空 settings 应不报错: %v", err)
	}
}

func TestUpdateRuntimeSettings_NotFound_Rejected(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	sc := 1.0
	_, err := svc.UpdateRuntimeSettings(testUserID, "nonexistent", &UpdateRuntimeSettingsRequest{Scale: &sc})
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func TestUpdateRuntimeSettings_RevisionCAS_Success(t *testing.T) {
	svc, db, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	rs := getRuntimeSettingsFromDB(t, db, inst.ID)
	expectedRev := rs.SettingsRevision
	sc := 1.5
	updated, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		Scale:            &sc,
		ExpectedRevision: &expectedRev,
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeSettings CAS: %v", err)
	}
	if updated.SettingsRevision != expectedRev+1 {
		t.Fatalf("SettingsRevision = %d, 期望 %d", updated.SettingsRevision, expectedRev+1)
	}
}

func TestUpdateRuntimeSettings_RevisionCAS_Conflict(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	staleRev := 999
	sc := 1.5
	_, err := svc.UpdateRuntimeSettings(testUserID, inst.ID, &UpdateRuntimeSettingsRequest{
		Scale:            &sc,
		ExpectedRevision: &staleRev,
	})
	assertInstallationError(t, err, ErrCodeRevisionConflict)
}

func TestUpdateRuntimeSettings_UserOwnership_Rejected(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	if err := svc.EnableInstallation(testUserID, inst.ID); err != nil {
		t.Fatalf("EnableInstallation: %v", err)
	}

	sc := 1.5
	_, err := svc.UpdateRuntimeSettings("wrong_user", inst.ID, &UpdateRuntimeSettingsRequest{Scale: &sc})
	assertInstallationError(t, err, ErrCodeInstallationInvalid)
}

func TestListInstallations_Success(t *testing.T) {
	svc, _, _, instA, instB, _ := setupTwoInstallations(t)

	items, err := svc.ListInstallations(testUserID)
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条安装记录, 实际 %d", len(items))
	}

	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[instA.ID] || !ids[instB.ID] {
		t.Fatalf("期望包含 %s 和 %s, 实际 %v", instA.ID, instB.ID, ids)
	}
}

func TestListInstallations_EmptyForOtherUser(t *testing.T) {
	svc, _, _, _, _, _ := setupTwoInstallations(t)

	items, err := svc.ListInstallations("other_user")
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("期望 0 条记录, 实际 %d", len(items))
	}
}

func TestGetInstallation_Success(t *testing.T) {
	svc, _, _, inst, _ := setupInstalledService(t)

	got, err := svc.GetInstallation(inst.ID)
	if err != nil {
		t.Fatalf("GetInstallation: %v", err)
	}
	if got.ID != inst.ID {
		t.Fatalf("ID = %s, 期望 %s", got.ID, inst.ID)
	}
}

func TestGetInstallation_NotFound(t *testing.T) {
	svc, _, _, _, _ := setupInstalledService(t)

	_, err := svc.GetInstallation("nonexistent")
	assertInstallationError(t, err, ErrCodeInstallationNotFound)
}

func contains(data []byte, substr string) bool {
	return len(data) >= len(substr) && (findBytes(data, []byte(substr)))
}

func findBytes(data, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(data)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if data[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
