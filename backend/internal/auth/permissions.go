// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package auth

const (
	PermSystemAdmin              = "system.admin"
	PermWorkbenchBrowserControl  = "workbench.browser.control"
	PermDesktopPetRead           = "desktop_pet.read"
	PermDesktopPetWrite          = "desktop_pet.write"
	PermDesktopPetGenerate       = "desktop_pet.generate"
	PermDesktopPetImport         = "desktop_pet.import"
	PermDesktopPetInstall        = "desktop_pet.install"
	PermDesktopPetRuntimeControl = "desktop_pet.runtime.control"
	PermDesktopPetBehaviorManage = "desktop_pet.behavior.manage"
	PermDesktopPetBehaviorAdmin  = "desktop_pet.behavior.admin"
	PermDesktopPetRepair         = "desktop_pet.repair"
	PermMaintenanceRead          = "desktop_pet.maintenance.read"
	PermBackupCreate             = "desktop_pet.backup.create"
	PermExportCreate             = "desktop_pet.export.create"
	PermSecurityAuditRead        = "security.audit.read"
	PermDoctorRun                = "doctor.run"
	PermDoctorRepair             = "doctor.repair"
	SystemShutdown               = "system.shutdown"
)

func StandardPermissions() []string {
	return []string{
		PermDesktopPetRead, PermDesktopPetWrite, PermDesktopPetGenerate,
		PermDesktopPetImport, PermDesktopPetInstall, PermDesktopPetRuntimeControl,
		PermDesktopPetBehaviorManage,
	}
}

// OwnerDevicePermissions are granted to a device credential paired to the
// personal Space. Fine-grained tool/skill/MCP authorization is still enforced
// by the Capability Gateway and permission broker at invocation time.
func OwnerDevicePermissions() []string {
	return append(StandardPermissions(),
		PermSystemAdmin, PermWorkbenchBrowserControl,
		PermDesktopPetBehaviorAdmin,
		PermDesktopPetRepair,
		PermMaintenanceRead,
		PermBackupCreate,
		PermExportCreate,
		PermSecurityAuditRead,
		PermDoctorRun,
		PermDoctorRepair,
		SystemShutdown,
	)
}

func SystemWorkerPermissions() []string {
	return []string{PermDesktopPetRead, PermDesktopPetWrite, PermDesktopPetGenerate, PermDesktopPetRuntimeControl, PermDesktopPetBehaviorManage}
}
func RepairPermissions() []string {
	return append(StandardPermissions(), PermDesktopPetRepair, PermDoctorRun, PermDoctorRepair)
}
