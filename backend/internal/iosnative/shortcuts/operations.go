package shortcuts

const (
	OperationStatus              = "shortcuts.status"
	OperationIntentRegister      = "shortcuts.intent.register"
	OperationIntentRevoke        = "shortcuts.intent.revoke"
	OperationIntentDonate        = "shortcuts.intent.donate"
	OperationEntitiesCharacters  = "shortcuts.entities.characters"
	OperationEntitiesConversations = "shortcuts.entities.conversations"
	OperationEntitiesAlarms      = "shortcuts.entities.alarms"
	OperationEntitiesReminders   = "shortcuts.entities.reminders"
	OperationEntitiesActions     = "shortcuts.entities.actions"
	OperationEntityResolve       = "shortcuts.entity.resolve"
	OperationEntitySuggestions   = "shortcuts.entity.suggestions"
	OperationActionsCatalog      = "shortcuts.actions.catalog"
	OperationActionDescribe      = "shortcuts.action.describe"
	OperationActionExecute       = "shortcuts.action.execute"
	OperationActionConfirm       = "shortcuts.action.confirm"
	OperationRuntimeReadiness    = "shortcuts.runtime.readiness"
	OperationRuntimeEnsure       = "shortcuts.runtime.ensure"
	OperationSnapshotGet         = "shortcuts.snapshot.get"
	OperationSnapshotRefresh     = "shortcuts.snapshot.refresh"
	OperationShortcutsProvider   = "shortcuts.shortcuts.provider"
	OperationShortcutsPhrase     = "shortcuts.shortcuts.phrase"
	OperationShortcutsUpdate     = "shortcuts.shortcuts.update"
	OperationSettingsGet         = "shortcuts.settings.get"
	OperationSettingsUpdate      = "shortcuts.settings.update"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationIntentRegister,
		OperationIntentRevoke,
		OperationIntentDonate,
		OperationEntitiesCharacters,
		OperationEntitiesConversations,
		OperationEntitiesAlarms,
		OperationEntitiesReminders,
		OperationEntitiesActions,
		OperationEntityResolve,
		OperationEntitySuggestions,
		OperationActionsCatalog,
		OperationActionDescribe,
		OperationActionExecute,
		OperationActionConfirm,
		OperationRuntimeReadiness,
		OperationRuntimeEnsure,
		OperationSnapshotGet,
		OperationSnapshotRefresh,
		OperationShortcutsProvider,
		OperationShortcutsPhrase,
		OperationShortcutsUpdate,
		OperationSettingsGet,
		OperationSettingsUpdate,
	}
}
