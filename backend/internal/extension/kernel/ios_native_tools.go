//go:build ios
// +build ios

package kernel

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/iosnative/alarms"
	"github.com/u-ai/backend/internal/iosnative/bluetooth"
	"github.com/u-ai/backend/internal/iosnative/calendar"
	"github.com/u-ai/backend/internal/iosnative/clipboard"
	"github.com/u-ai/backend/internal/iosnative/contacts"
	"github.com/u-ai/backend/internal/iosnative/health"
	"github.com/u-ai/backend/internal/iosnative/homekit"
	"github.com/u-ai/backend/internal/iosnative/media"
	"github.com/u-ai/backend/internal/iosnative/reminders"
	"github.com/u-ai/backend/internal/iosnative/share"
	"github.com/u-ai/backend/internal/iosnative/shortcuts"
)

func registerIOSToolsIfPresent(toolRegistry *capability.ToolRegistry, provider capability.IOSProvider) error {
	if provider == nil {
		return nil
	}
	defs := buildIOSToolDefinitions()
	for _, def := range defs {
		if err := toolRegistry.Register(nil, def); err != nil {
			return fmt.Errorf("register ios tool %s: %w", def.ID, err)
		}
	}
	return nil
}

func buildIOSToolDefinitions() []capability.ToolDefinition {
	genericInput := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	genericOutput := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	statusOutput := json.RawMessage(`{"type":"object","properties":{"available":{"type":"boolean"},"authorized":{"type":"boolean"},"message":{"type":"string"}}}`)

	rt := capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeIOS_Native}

	defs := []capability.ToolDefinition{
		// === HEALTH (9 tools) ===
		makeIOSNativeTool(rt, health.ToolIDAuthorizationStatus, health.ModelNameAuthorizationStatus, health.OpHealthAuthorizationStatus, "Health Authorization Status", "Query HealthKit authorization status", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDAuthorizationRequest, health.ModelNameAuthorizationRequest, health.OpHealthAuthorizationRequest, "Request Health Authorization", "Request HealthKit sharing permission", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDProfileRead, health.ModelNameProfileRead, health.OpHealthProfileRead, "Read Health Profile", "Read user health profile information", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDSamplesQuery, health.ModelNameSamplesQuery, health.OpHealthSamplesQuery, "Query Health Samples", "Query health quantity samples", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDStatisticsQuery, health.ModelNameStatisticsQuery, health.OpHealthStatisticsQuery, "Query Health Statistics", "Query accumulated health statistics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDWorkoutsQuery, health.ModelNameWorkoutsQuery, health.OpHealthWorkoutsQuery, "Query Health Workouts", "Query workout sessions", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDWorkoutsDetail, health.ModelNameWorkoutsDetail, health.OpHealthWorkoutsDetail, "Get Workout Detail", "Get detailed workout information", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDSleepQuery, health.ModelNameSleepQuery, health.OpHealthSleepQuery, "Query Sleep Data", "Query sleep analysis data", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, health.ToolIDActivityQuery, health.ModelNameActivityQuery, health.OpHealthActivityQuery, "Query Activity Summary", "Query daily activity summary", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),

		// === CALENDAR (9 tools) ===
		makeIOSNativeTool(rt, calendar.ToolIDStatus, calendar.ModelNameStatus, calendar.OperationStatus, "Calendar Status", "Query EventKit availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, calendar.ToolIDAuthorizationStatus, calendar.ModelNameAuthorizationStatus, calendar.OperationAuthorizationStatus, "Calendar Authorization Status", "Query calendar authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, calendar.ToolIDAuthorizationRequest, calendar.ModelNameAuthorizationRequest, calendar.OperationAuthorizationRequest, "Request Calendar Authorization", "Request calendar access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, calendar.ToolIDCalendarsList, calendar.ModelNameCalendarsList, calendar.OperationCalendarsList, "List Calendars", "List available calendars", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, calendar.ToolIDEventsQuery, calendar.ModelNameEventsQuery, calendar.OperationEventsQuery, "Query Calendar Events", "Query events in a date range", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, calendar.ToolIDEventsGet, calendar.ModelNameEventsGet, calendar.OperationEventsGet, "Get Calendar Event", "Get a single calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, calendar.ToolIDEventsCreate, calendar.ModelNameEventsCreate, calendar.OperationEventsCreate, "Create Calendar Event", "Create a new calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, calendar.ToolIDEventsUpdate, calendar.ModelNameEventsUpdate, calendar.OperationEventsUpdate, "Update Calendar Event", "Update an existing calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, calendar.ToolIDEventsDelete, calendar.ModelNameEventsDelete, calendar.OperationEventsDelete, "Delete Calendar Event", "Delete a calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === REMINDERS (11 tools) ===
		makeIOSNativeTool(rt, reminders.ToolIDStatus, reminders.ModelNameStatus, reminders.OperationStatus, "Reminders Status", "Query reminders availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, reminders.ToolIDAuthorizationStatus, reminders.ModelNameAuthorizationStatus, reminders.OperationAuthorizationStatus, "Reminders Authorization Status", "Query reminders authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, reminders.ToolIDAuthorizationRequest, reminders.ModelNameAuthorizationRequest, reminders.OperationAuthorizationRequest, "Request Reminders Authorization", "Request reminders access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, reminders.ToolIDListsList, reminders.ModelNameListsList, reminders.OperationListsList, "List Reminder Lists", "List reminder lists", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, reminders.ToolIDQuery, reminders.ModelNameQuery, reminders.OperationQuery, "Query Reminders", "Query reminders by predicate", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, reminders.ToolIDGet, reminders.ModelNameGet, reminders.OperationGet, "Get Reminder", "Get a single reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, reminders.ToolIDCreate, reminders.ModelNameCreate, reminders.OperationCreate, "Create Reminder", "Create a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, reminders.ToolIDUpdate, reminders.ModelNameUpdate, reminders.OperationUpdate, "Update Reminder", "Update a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, reminders.ToolIDComplete, reminders.ModelNameComplete, reminders.OperationComplete, "Complete Reminder", "Mark a reminder as completed", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, reminders.ToolIDUncomplete, reminders.ModelNameUncomplete, reminders.OperationUncomplete, "Uncomplete Reminder", "Mark a reminder as not completed", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, reminders.ToolIDDelete, reminders.ModelNameDelete, reminders.OperationDelete, "Delete Reminder", "Delete a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === CONTACTS (14 tools) ===
		makeIOSNativeTool(rt, contacts.ToolIDStatus, contacts.ModelNameContacts, contacts.OperationStatus, "Contacts Status", "Query contacts availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDAuthorizationStatus, contacts.ModelNameContacts, contacts.OperationAuthorizationStatus, "Contacts Authorization Status", "Query contacts authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDAuthorizationRequest, contacts.ModelNameContacts, contacts.OperationAuthorizationRequest, "Request Contacts Authorization", "Request contacts access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDSearch, contacts.ModelNameContacts, contacts.OperationSearch, "Search Contacts", "Search contacts by field", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDList, contacts.ModelNameContacts, contacts.OperationList, "List Contacts", "List contacts with pagination", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDGet, contacts.ModelNameContacts, contacts.OperationGet, "Get Contact", "Get contact details", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDCreate, contacts.ModelNameContacts, contacts.OperationCreate, "Create Contact", "Create a new contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, contacts.ToolIDUpdate, contacts.ModelNameContacts, contacts.OperationUpdate, "Update Contact", "Update a contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, contacts.ToolIDDelete, contacts.ModelNameContacts, contacts.OperationDelete, "Delete Contact", "Delete a contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, contacts.ToolIDContainersList, contacts.ModelNameContacts, contacts.OperationContainersList, "List Contact Containers", "List contact containers", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDGroupsList, contacts.ModelNameContacts, contacts.OperationGroupsList, "List Contact Groups", "List contact groups", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDPhotoGet, contacts.ModelNameContacts, contacts.OperationPhotoGet, "Get Contact Photo", "Get contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, contacts.ToolIDPhotoSet, contacts.ModelNameContacts, contacts.OperationPhotoSet, "Set Contact Photo", "Set contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, contacts.ToolIDPhotoRemove, contacts.ModelNameContacts, contacts.OperationPhotoRemove, "Remove Contact Photo", "Remove contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),

		// === HOMEKIT (15 tools) ===
		makeIOSNativeTool(rt, homekit.ToolIDStatus, homekit.ModelNameHomeKit, homekit.OperationStatus, "HomeKit Status", "Query HomeKit availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDHomesList, homekit.ModelNameHomeKit, homekit.OperationHomesList, "List HomeKit Homes", "List HomeKit homes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDHomesGet, homekit.ModelNameHomeKit, homekit.OperationHomesGet, "Get HomeKit Home", "Get a HomeKit home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDRoomsList, homekit.ModelNameHomeKit, homekit.OperationRoomsList, "List HomeKit Rooms", "List rooms in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDZonesList, homekit.ModelNameHomeKit, homekit.OperationZonesList, "List HomeKit Zones", "List zones in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDAccessoriesList, homekit.ModelNameHomeKit, homekit.OperationAccessoriesList, "List HomeKit Accessories", "List accessories in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDAccessoriesGet, homekit.ModelNameHomeKit, homekit.OperationAccessoriesGet, "Get HomeKit Accessory", "Get an accessory", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDServicesList, homekit.ModelNameHomeKit, homekit.OperationServicesList, "List HomeKit Services", "List services on an accessory", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDCharacteristicsList, homekit.ModelNameHomeKit, homekit.OperationCharacteristicsList, "List HomeKit Characteristics", "List characteristics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDCharacteristicsRead, homekit.ModelNameHomeKit, homekit.OperationCharacteristicsRead, "Read HomeKit Characteristic", "Read a characteristic value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDCharacteristicsWrite, homekit.ModelNameHomeKit, homekit.OperationCharacteristicsWrite, "Write HomeKit Characteristic", "Write a characteristic value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, homekit.ToolIDScenesList, homekit.ModelNameHomeKit, homekit.OperationScenesList, "List HomeKit Scenes", "List scenes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDScenesGet, homekit.ModelNameHomeKit, homekit.OperationScenesGet, "Get HomeKit Scene", "Get a scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, homekit.ToolIDScenesExecute, homekit.ModelNameHomeKit, homekit.OperationScenesExecute, "Execute HomeKit Scene", "Execute a HomeKit scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, homekit.ToolIDAutomationsList, homekit.ModelNameHomeKit, homekit.OperationAutomationsList, "List HomeKit Automations", "List automations", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),

		// === BLUETOOTH (19 tools) ===
		makeIOSNativeTool(rt, bluetooth.ToolIDStatus, "", bluetooth.OperationStatus, "Bluetooth Status", "Query Bluetooth availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDScanStart, "", bluetooth.OperationScanStart, "Start Bluetooth Scan", "Start scanning for peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDScanStop, "", bluetooth.OperationScanStop, "Stop Bluetooth Scan", "Stop scanning for peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDPeripheralGet, "", bluetooth.OperationPeripheralGet, "Get Bluetooth Peripheral", "Get a discovered peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDPeripheralConnected, "", bluetooth.OperationPeripheralConnected, "List Connected Peripherals", "List connected peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDConnect, "", bluetooth.OperationConnect, "Connect Bluetooth", "Connect to a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDDisconnect, "", bluetooth.OperationDisconnect, "Disconnect Bluetooth", "Disconnect from a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDServicesDiscover, "", bluetooth.OperationServicesDiscover, "Discover BLE Services", "Discover services on a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDCharacteristicsDiscover, "", bluetooth.OperationCharacteristicsDiscover, "Discover BLE Characteristics", "Discover characteristics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDDescriptorsDiscover, "", bluetooth.OperationDescriptorsDiscover, "Discover BLE Descriptors", "Discover descriptors", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDCharacteristicRead, "", bluetooth.OperationCharacteristicRead, "Read BLE Characteristic", "Read a BLE characteristic", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDCharacteristicWrite, "", bluetooth.OperationCharacteristicWrite, "Write BLE Characteristic", "Write a BLE characteristic", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDCharacteristicSubscribe, "", bluetooth.OperationCharacteristicSubscribe, "Subscribe BLE Characteristic", "Subscribe to characteristic notifications", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDCharacteristicUnsubscribe, "", bluetooth.OperationCharacteristicUnsubscribe, "Unsubscribe BLE Characteristic", "Unsubscribe from notifications", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDDescriptorRead, "", bluetooth.OperationDescriptorRead, "Read BLE Descriptor", "Read a BLE descriptor", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDDescriptorWrite, "", bluetooth.OperationDescriptorWrite, "Write BLE Descriptor", "Write a BLE descriptor", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDRSSIRead, "", bluetooth.OperationRSSIRead, "Read BLE RSSI", "Read RSSI value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSNativeTool(rt, bluetooth.ToolIDPeripheralRoleStart, "", bluetooth.OperationPeripheralRoleStart, "Start Peripheral Role", "Start advertising as peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, bluetooth.ToolIDPeripheralRoleStop, "", bluetooth.OperationPeripheralRoleStop, "Stop Peripheral Role", "Stop advertising as peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),

		// === SHORTCUTS (20 tools) ===
		makeIOSNativeTool(rt, shortcuts.ToolIDStatus, "", shortcuts.OperationStatus, "Shortcuts Status", "Query Shortcuts availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitiesCharacters, "", shortcuts.OperationEntitiesCharacters, "Get Shortcut Entities Characters", "Get shortcut character entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitiesConversations, "", shortcuts.OperationEntitiesConversations, "Get Shortcut Entities Conversations", "Get shortcut conversation entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitiesAlarms, "", shortcuts.OperationEntitiesAlarms, "Get Shortcut Entities Alarms", "Get shortcut alarm entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitiesReminders, "", shortcuts.OperationEntitiesReminders, "Get Shortcut Entities Reminders", "Get shortcut reminder entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitiesActions, "", shortcuts.OperationEntitiesActions, "Get Shortcut Entities Actions", "Get shortcut action entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntityResolve, "", shortcuts.OperationEntityResolve, "Resolve Shortcut Entity", "Resolve a shortcut entity", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDEntitySuggestions, "", shortcuts.OperationEntitySuggestions, "Get Shortcut Entity Suggestions", "Get shortcut entity suggestions", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDActionsCatalog, "", shortcuts.OperationActionsCatalog, "Get Shortcut Actions Catalog", "Get shortcut actions catalog", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDActionDescribe, "", shortcuts.OperationActionDescribe, "Describe Shortcut Action", "Describe a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDActionExecute, "", shortcuts.OperationActionExecute, "Execute Shortcut Action", "Execute a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, shortcuts.ToolIDActionConfirm, "", shortcuts.OperationActionConfirm, "Confirm Shortcut Action", "Confirm a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, shortcuts.ToolIDRuntimeReadiness, "", shortcuts.OperationRuntimeReadiness, "Shortcuts Runtime Readiness", "Check shortcuts runtime readiness", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDRuntimeEnsure, "", shortcuts.OperationRuntimeEnsure, "Shortcuts Runtime Ensure", "Ensure shortcuts runtime ready", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSNativeTool(rt, shortcuts.ToolIDSnapshotGet, "", shortcuts.OperationSnapshotGet, "Get Shortcuts Snapshot", "Get shortcuts snapshot", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDSnapshotRefresh, "", shortcuts.OperationSnapshotRefresh, "Refresh Shortcuts Snapshot", "Refresh shortcuts snapshot", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, shortcuts.ToolIDShortcutsProvider, "", shortcuts.OperationShortcutsProvider, "Get Shortcuts Provider", "Get shortcuts provider info", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDShortcutsPhrase, "", shortcuts.OperationShortcutsPhrase, "Get Shortcut Phrase", "Get shortcut phrase", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDSettingsGet, "", shortcuts.OperationSettingsGet, "Get Shortcuts Settings", "Get shortcuts settings", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, shortcuts.ToolIDSettingsUpdate, "", shortcuts.OperationSettingsUpdate, "Update Shortcuts Settings", "Update shortcuts settings", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),

		// === CLIPBOARD (5 tools) ===
		makeIOSNativeTool(rt, clipboard.ToolIDStatus, "", clipboard.OperationStatus, "Clipboard Status", "Query clipboard availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, clipboard.ToolIDDetect, "", clipboard.OperationDetect, "Detect Clipboard", "Detect clipboard changes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, clipboard.ToolIDRead, "", clipboard.OperationRead, "Read Clipboard", "Read clipboard content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, clipboard.ToolIDWrite, "", clipboard.OperationWrite, "Write Clipboard", "Write to clipboard", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, clipboard.ToolIDClear, "", clipboard.OperationClear, "Clear Clipboard", "Clear clipboard content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === MEDIA (15 tools) ===
		makeIOSNativeTool(rt, media.ToolIDStatus, "", media.OperationStatus, "Media Status", "Query media availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, media.ToolIDPhotosPick, "", media.OperationPhotosPick, "Pick Photos", "Pick photos from library", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, media.ToolIDPhotosStatus, "", media.OperationPhotosStatus, "Photos Status", "Query photos authorization status", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, media.ToolIDPhotosList, "", media.OperationPhotosList, "List Photos", "List photo assets", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, media.ToolIDPhotosGet, "", media.OperationPhotosGet, "Get Photo", "Get a photo asset", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, media.ToolIDPhotosExport, "", media.OperationPhotosExport, "Export Photo", "Export a photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDPhotosSave, "", media.OperationPhotosSave, "Save Photo", "Save a photo to library", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDPhotosDelete, "", media.OperationPhotosDelete, "Delete Photo", "Delete a photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDPhotosManageLimited, "", media.OperationPhotosManageLimited, "Manage Limited Photos Access", "Manage limited photos access", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDCameraStatus, "", media.OperationCameraStatus, "Camera Status", "Query camera availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, media.ToolIDCameraDevices, "", media.OperationCameraDevices, "List Camera Devices", "List available cameras", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, media.ToolIDCameraCapturePhoto, "", media.OperationCameraCapturePhoto, "Capture Photo", "Capture a photo with camera", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDCameraRecordVideo, "", media.OperationCameraRecordVideo, "Record Video", "Record a video with camera", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, media.ToolIDAudioStatus, "", media.OperationAudioStatus, "Audio Status", "Query audio recorder availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, media.ToolIDAudioRecord, "", media.OperationAudioRecord, "Record Audio", "Record audio", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),

		// === ALARMS (11 tools) ===
		makeIOSNativeTool(rt, alarms.ToolIDStatus, "", alarms.OperationStatus, "Alarms Status", "Query alarms availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, alarms.ToolIDAuthorizationStatus, "", alarms.OperationAuthorizationStatus, "Alarms Authorization Status", "Query alarms authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, alarms.ToolIDAuthorizationRequest, "", alarms.OperationAuthorizationRequest, "Request Alarms Authorization", "Request alarms access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSNativeTool(rt, alarms.ToolIDList, "", alarms.OperationList, "List Alarms", "List all alarms", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, alarms.ToolIDGet, "", alarms.OperationGet, "Get Alarm", "Get an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSNativeTool(rt, alarms.ToolIDSchedule, "", alarms.OperationSchedule, "Schedule Alarm", "Schedule an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, alarms.ToolIDStop, "", alarms.OperationStop, "Stop Alarm", "Stop an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, alarms.ToolIDCancel, "", alarms.OperationCancel, "Cancel Alarm", "Cancel an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, alarms.ToolIDCountdown, "", alarms.OperationCountdown, "Start Countdown", "Start a countdown", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, alarms.ToolIDPause, "", alarms.OperationPause, "Pause Alarm", "Pause an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSNativeTool(rt, alarms.ToolIDResume, "", alarms.OperationResume, "Resume Alarm", "Resume an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),

	// === SHARE (5 tools) ===
	makeIOSNativeTool(rt, share.ToolIDStatus, "", share.OperationStatus, "Share Status", "Query share availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
	makeIOSNativeTool(rt, share.ToolIDSend, "", share.OperationSend, "Send Share", "Send share content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
	makeIOSNativeTool(rt, share.ToolIDPreviewSupported, "", share.OperationPreviewSupported, "Preview Supported", "Check if preview is supported", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
	makeIOSNativeTool(rt, share.ToolIDStagingCleanup, "", share.OperationStagingCleanup, "Cleanup Share Staging", "Cleanup share staging area", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
	makeIOSNativeTool(rt, share.ToolIDLimitedDelete, "", share.OperationLimitedDelete, "Delete Limited Share Item", "Delete a limited share item", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
	}
	return defs
}

func makeIOSNativeTool(rt capability.RuntimeBinding, toolID, modelName, operation, name, description string, inputSchema, outputSchema json.RawMessage, permissions []capability.PermissionRequirement, riskLevel capability.RiskLevel, hasSideEffects bool) capability.ToolDefinition {
	rt.HandlerName = operation
	return capability.ToolDefinition{
		ID:             toolID,
		ModelName:      modelName,
		Source:         capability.ToolSourceBuiltin,
		Name:           name,
		Description:    description,
		InputSchema:    inputSchema,
		OutputSchema:   outputSchema,
		Permissions:    permissions,
		RiskLevel:      riskLevel,
		HasSideEffects: hasSideEffects,
		Enabled:        true,
		Runtime:        rt,
	}
}
