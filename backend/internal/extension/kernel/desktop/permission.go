package desktop

var DesktopPermissionDefinitions = []PermissionDef{
	{ID: "desktop.menu.register", Name: "Register Menu Items", Category: "desktop", RiskLevel: "low", Description: "Register application menu items"},
	{ID: "desktop.menu.execute", Name: "Execute Menu Actions", Category: "desktop", RiskLevel: "medium", Description: "Execute menu item actions"},
	{ID: "desktop.tray.register", Name: "Register Tray Items", Category: "desktop", RiskLevel: "low", Description: "Register system tray items"},
	{ID: "desktop.tray.execute", Name: "Execute Tray Actions", Category: "desktop", RiskLevel: "medium", Description: "Execute tray item actions"},
	{ID: "desktop.shortcut.application.register", Name: "Register Application Shortcuts", Category: "desktop", RiskLevel: "low", Description: "Register application-scoped keyboard shortcuts"},
	{ID: "desktop.shortcut.application.execute", Name: "Execute Application Shortcuts", Category: "desktop", RiskLevel: "medium", Description: "Execute application shortcut actions"},
	{ID: "desktop.shortcut.global.register", Name: "Register Global Shortcuts", Category: "desktop", RiskLevel: "high", Description: "Register system-wide global keyboard shortcuts"},
	{ID: "desktop.shortcut.global.execute", Name: "Execute Global Shortcuts", Category: "desktop", RiskLevel: "high", Description: "Execute global shortcut actions"},
	{ID: "desktop.navigation", Name: "Desktop Navigation", Category: "desktop", RiskLevel: "medium", Description: "Navigate to application pages"},
	{ID: "desktop.dialog.open", Name: "Open Dialogs", Category: "desktop", RiskLevel: "medium", Description: "Open application dialog windows"},
}

type PermissionDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	RiskLevel   string `json:"riskLevel"`
	Description string `json:"description"`
}

func GetDesktopPermissionDefs() []PermissionDef {
	return DesktopPermissionDefinitions
}

func IsDesktopPermission(permissionID string) bool {
	for _, p := range DesktopPermissionDefinitions {
		if p.ID == permissionID {
			return true
		}
	}
	return false
}

func IsHighRiskDesktopPermission(permissionID string) bool {
	for _, p := range DesktopPermissionDefinitions {
		if p.ID == permissionID && p.RiskLevel == "high" {
			return true
		}
	}
	return false
}
