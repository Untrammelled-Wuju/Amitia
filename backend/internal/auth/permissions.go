// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package auth

const (
	PermDesktopPetRead           = "desktop_pet.read"
	PermDesktopPetWrite          = "desktop_pet.write"
	PermDesktopPetGenerate       = "desktop_pet.generate"
	PermDesktopPetImport         = "desktop_pet.import"
	PermDesktopPetInstall        = "desktop_pet.install"
	PermDesktopPetRuntimeControl = "desktop_pet.runtime.control"
	PermDesktopPetBehaviorManage = "desktop_pet.behavior.manage"
	PermDesktopPetBehaviorAdmin  = "desktop_pet.behavior.admin"
	PermDesktopPetMigrate        = "desktop_pet.migrate"
	PermDesktopPetRepair         = "desktop_pet.repair"
	PermSecurityAuditRead        = "security.audit.read"
	PermDoctorRun                = "doctor.run"
	PermDoctorRepair             = "doctor.repair"
	SystemShutdown               = "system.shutdown"
)

func DefaultUserPermissions() []string {
	return []string{
		PermDesktopPetRead,
		PermDesktopPetWrite,
		PermDesktopPetGenerate,
		PermDesktopPetImport,
		PermDesktopPetInstall,
		PermDesktopPetRuntimeControl,
		PermDesktopPetBehaviorManage,
	}
}

func AdminPermissions() []string {
	return append(DefaultUserPermissions(),
		PermDesktopPetBehaviorAdmin,
		PermDesktopPetMigrate,
		PermDesktopPetRepair,
		PermSecurityAuditRead,
		PermDoctorRun,
		PermDoctorRepair,
		SystemShutdown,
	)
}

func SystemWorkerPermissions() []string {
	return []string{
		PermDesktopPetRead,
		PermDesktopPetWrite,
		PermDesktopPetGenerate,
		PermDesktopPetRuntimeControl,
		PermDesktopPetBehaviorManage,
	}
}

func MigrationPermissions() []string {
	return []string{
		PermDesktopPetRead,
		PermDesktopPetWrite,
		PermDesktopPetMigrate,
	}
}

func RepairPermissions() []string {
	return append(DefaultUserPermissions(),
		PermDesktopPetRepair,
		PermDoctorRun,
		PermDoctorRepair,
	)
}
