//go:build ios
// +build ios

package kernel

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerIOSToolsIfPresent(toolRegistry *capability.ToolRegistry, provider capability.IOSProvider) error {
	if provider == nil {
		return nil
	}
	defs := buildIOSToolDefinitions()
	for _, def := range defs {
		if err := toolRegistry.Register(nil, def); err != nil {
			if err := toolRegistry.Replace(nil, def); err != nil {
				return fmt.Errorf("register ios tool %s: %w", def.ID, err)
			}
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
		makeIOSTool(rt, "health", "authorization.status", "Health Authorization Status", "Query HealthKit authorization status", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "authorization.request", "Request Health Authorization", "Request HealthKit sharing permission", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "profile.read", "Read Health Profile", "Read user health profile information", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "samples.query", "Query Health Samples", "Query health quantity samples", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "statistics.query", "Query Health Statistics", "Query accumulated health statistics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "workouts.query", "Query Health Workouts", "Query workout sessions", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "workouts.detail", "Get Workout Detail", "Get detailed workout information", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "sleep.query", "Query Sleep Data", "Query sleep analysis data", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "health", "activity.query", "Query Activity Summary", "Query daily activity summary", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.health.read", Risk: "medium"}}, capability.RiskMedium, false),

		// === CALENDAR (9 tools) ===
		makeIOSTool(rt, "calendar", "status", "Calendar Status", "Query EventKit availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "calendar", "authorization.status", "Calendar Authorization Status", "Query calendar authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "calendar", "authorization.request", "Request Calendar Authorization", "Request calendar access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "calendar", "calendars.list", "List Calendars", "List available calendars", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "calendar", "events.query", "Query Calendar Events", "Query events in a date range", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "calendar", "events.get", "Get Calendar Event", "Get a single calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "calendar", "events.create", "Create Calendar Event", "Create a new calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "calendar", "events.update", "Update Calendar Event", "Update an existing calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "calendar", "events.delete", "Delete Calendar Event", "Delete a calendar event", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.calendar.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === REMINDERS (11 tools) ===
		makeIOSTool(rt, "reminders", "status", "Reminders Status", "Query reminders availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "reminders", "authorization.status", "Reminders Authorization Status", "Query reminders authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "reminders", "authorization.request", "Request Reminders Authorization", "Request reminders access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "reminders", "lists.list", "List Reminder Lists", "List reminder lists", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "reminders", "query", "Query Reminders", "Query reminders by predicate", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "reminders", "get", "Get Reminder", "Get a single reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "reminders", "create", "Create Reminder", "Create a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "reminders", "update", "Update Reminder", "Update a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "reminders", "complete", "Complete Reminder", "Mark a reminder as completed", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "reminders", "uncomplete", "Uncomplete Reminder", "Mark a reminder as not completed", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "reminders", "delete", "Delete Reminder", "Delete a reminder", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.reminders.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === CONTACTS (14 tools) ===
		makeIOSTool(rt, "contacts", "status", "Contacts Status", "Query contacts availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "authorization.status", "Contacts Authorization Status", "Query contacts authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "authorization.request", "Request Contacts Authorization", "Request contacts access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "search", "Search Contacts", "Search contacts by field", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "list", "List Contacts", "List contacts with pagination", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "get", "Get Contact", "Get contact details", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "create", "Create Contact", "Create a new contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "contacts", "update", "Update Contact", "Update a contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "contacts", "delete", "Delete Contact", "Delete a contact", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "contacts", "containers.list", "List Contact Containers", "List contact containers", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "groups.list", "List Contact Groups", "List contact groups", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "photo.get", "Get Contact Photo", "Get contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "contacts", "photo.set", "Set Contact Photo", "Set contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "contacts", "photo.remove", "Remove Contact Photo", "Remove contact photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.contacts.write", Risk: "high"}}, capability.RiskHigh, true),

		// === HOMEKIT (25 tools) ===
		makeIOSTool(rt, "homekit", "status", "HomeKit Status", "Query HomeKit availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "homes.list", "List HomeKit Homes", "List HomeKit homes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "homes.get", "Get HomeKit Home", "Get a HomeKit home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "rooms.list", "List HomeKit Rooms", "List rooms in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "zones.list", "List HomeKit Zones", "List zones in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "accessories.list", "List HomeKit Accessories", "List accessories in a home", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "accessories.get", "Get HomeKit Accessory", "Get an accessory", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "services.list", "List HomeKit Services", "List services on an accessory", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "characteristics.list", "List HomeKit Characteristics", "List characteristics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "characteristics.read", "Read HomeKit Characteristic", "Read a characteristic value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "characteristics.write", "Write HomeKit Characteristic", "Write a characteristic value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "scenes.list", "List HomeKit Scenes", "List scenes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "scenes.get", "Get HomeKit Scene", "Get a scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "scenes.execute", "Execute HomeKit Scene", "Execute a HomeKit scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "scenes.create", "Create HomeKit Scene", "Create a scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "scenes.update", "Update HomeKit Scene", "Update a scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "scenes.delete", "Delete HomeKit Scene", "Delete a scene", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "automations.list", "List HomeKit Automations", "List automations", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "automations.get", "Get HomeKit Automation", "Get an automation", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "homekit", "automations.create", "Create HomeKit Automation", "Create an automation", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "automations.update", "Update HomeKit Automation", "Update an automation", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "automations.enable", "Enable HomeKit Automation", "Enable an automation", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "automations.delete", "Delete HomeKit Automation", "Delete an automation", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "setup.present", "Present HomeKit Setup", "Present HomeKit setup UI", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "homekit", "enable", "Enable HomeKit", "Enable HomeKit integration", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.homekit.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === BLUETOOTH (19 tools) ===
		makeIOSTool(rt, "bluetooth", "status", "Bluetooth Status", "Query Bluetooth availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "bluetooth", "scan.start", "Start Bluetooth Scan", "Start scanning for peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "bluetooth", "scan.stop", "Stop Bluetooth Scan", "Stop scanning for peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "bluetooth", "peripheral.get", "Get Bluetooth Peripheral", "Get a discovered peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "bluetooth", "peripheral.connected", "List Connected Peripherals", "List connected peripherals", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.scan", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "bluetooth", "connect", "Connect Bluetooth", "Connect to a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "disconnect", "Disconnect Bluetooth", "Disconnect from a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "services.discover", "Discover BLE Services", "Discover services on a peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "characteristics.discover", "Discover BLE Characteristics", "Discover characteristics", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "descriptors.discover", "Discover BLE Descriptors", "Discover descriptors", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "characteristic.read", "Read BLE Characteristic", "Read a BLE characteristic", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "characteristic.write", "Write BLE Characteristic", "Write a BLE characteristic", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "characteristic.subscribe", "Subscribe BLE Characteristic", "Subscribe to characteristic notifications", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "characteristic.unsubscribe", "Unsubscribe BLE Characteristic", "Unsubscribe from notifications", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "descriptor.read", "Read BLE Descriptor", "Read a BLE descriptor", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "descriptor.write", "Write BLE Descriptor", "Write a BLE descriptor", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "rssi.read", "Read BLE RSSI", "Read RSSI value", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, false),
		makeIOSTool(rt, "bluetooth", "peripheral_role.start", "Start Peripheral Role", "Start advertising as peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "bluetooth", "peripheral_role.stop", "Stop Peripheral Role", "Stop advertising as peripheral", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.bluetooth.connect", Risk: "high"}}, capability.RiskHigh, true),

		// === SHORTCUTS (24 tools) ===
		makeIOSTool(rt, "shortcuts", "status", "Shortcuts Status", "Query Shortcuts availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "intent.register", "Register Shortcut Intent", "Register a shortcut intent", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "intent.revoke", "Revoke Shortcut Intent", "Revoke a shortcut intent", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "intent.donate", "Donate Shortcut Intent", "Donate a shortcut intent", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "entities.characters", "Get Shortcut Entities Characters", "Get shortcut character entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entities.conversations", "Get Shortcut Entities Conversations", "Get shortcut conversation entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entities.alarms", "Get Shortcut Entities Alarms", "Get shortcut alarm entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entities.reminders", "Get Shortcut Entities Reminders", "Get shortcut reminder entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entities.actions", "Get Shortcut Entities Actions", "Get shortcut action entities", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entity.resolve", "Resolve Shortcut Entity", "Resolve a shortcut entity", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "entity.suggestions", "Get Shortcut Entity Suggestions", "Get shortcut entity suggestions", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "actions.catalog", "Get Shortcut Actions Catalog", "Get shortcut actions catalog", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "action.describe", "Describe Shortcut Action", "Describe a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "action.execute", "Execute Shortcut Action", "Execute a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "action.confirm", "Confirm Shortcut Action", "Confirm a shortcut action", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "runtime.readiness", "Shortcuts Runtime Readiness", "Check shortcuts runtime readiness", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "runtime.ensure", "Shortcuts Runtime Ensure", "Ensure shortcuts runtime ready", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "snapshot.get", "Get Shortcuts Snapshot", "Get shortcuts snapshot", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "snapshot.refresh", "Refresh Shortcuts Snapshot", "Refresh shortcuts snapshot", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "shortcuts", "shortcuts.provider", "Get Shortcuts Provider", "Get shortcuts provider info", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "shortcuts.phrase", "Get Shortcut Phrase", "Get shortcut phrase", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "shortcuts.update", "Update Shortcuts", "Update shortcuts", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),
		makeIOSTool(rt, "shortcuts", "settings.get", "Get Shortcuts Settings", "Get shortcuts settings", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "shortcuts", "settings.update", "Update Shortcuts Settings", "Update shortcuts settings", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.shortcuts.execute", Risk: "high"}}, capability.RiskHigh, true),

		// === CLIPBOARD (5 tools) ===
		makeIOSTool(rt, "clipboard", "status", "Clipboard Status", "Query clipboard availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "clipboard", "detect", "Detect Clipboard", "Detect clipboard changes", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "clipboard", "read", "Read Clipboard", "Read clipboard content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "clipboard", "write", "Write Clipboard", "Write to clipboard", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "clipboard", "clear", "Clear Clipboard", "Clear clipboard content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.clipboard.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === MEDIA (15 tools) ===
		makeIOSTool(rt, "media", "status", "Media Status", "Query media availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "media", "photos.pick", "Pick Photos", "Pick photos from library", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "media", "photos.status", "Photos Status", "Query photos authorization status", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "media", "photos.list", "List Photos", "List photo assets", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "media", "photos.get", "Get Photo", "Get a photo asset", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "media", "photos.export", "Export Photo", "Export a photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "photos.save", "Save Photo", "Save a photo to library", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "photos.delete", "Delete Photo", "Delete a photo", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "photos.manage_limited_access", "Manage Limited Photos Access", "Manage limited photos access", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.photos", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "camera.status", "Camera Status", "Query camera availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "media", "camera.devices", "List Camera Devices", "List available cameras", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "media", "camera.capture_photo", "Capture Photo", "Capture a photo with camera", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "camera.record_video", "Record Video", "Record a video with camera", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "media", "audio.status", "Audio Status", "Query audio recorder availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "media", "audio.record", "Record Audio", "Record audio", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.media.camera", Risk: "medium"}}, capability.RiskMedium, true),

		// === ALARMS (11 tools) ===
		makeIOSTool(rt, "alarms", "status", "Alarms Status", "Query alarms availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "alarms", "authorization.status", "Alarms Authorization Status", "Query alarms authorization", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "alarms", "authorization.request", "Request Alarms Authorization", "Request alarms access", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "medium"}}, capability.RiskMedium, false),
		makeIOSTool(rt, "alarms", "list", "List Alarms", "List all alarms", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "alarms", "get", "Get Alarm", "Get an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.read", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "alarms", "schedule", "Schedule Alarm", "Schedule an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "alarms", "stop", "Stop Alarm", "Stop an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "alarms", "cancel", "Cancel Alarm", "Cancel an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "alarms", "countdown", "Start Countdown", "Start a countdown", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "alarms", "pause", "Pause Alarm", "Pause an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "alarms", "resume", "Resume Alarm", "Resume an alarm", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.alarms.write", Risk: "medium"}}, capability.RiskMedium, true),

		// === SHARE (9 tools) ===
		makeIOSTool(rt, "share", "status", "Share Status", "Query share availability", genericInput, statusOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "share", "send", "Send Share", "Send share content", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "share", "preview.supported", "Preview Supported", "Check if preview is supported", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "share", "receive.pending", "Get Pending Received Items", "Get pending received share items", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "share", "receive.consume", "Consume Received Item", "Consume a received share item", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "share", "receive.peek", "Peek Received Item", "Peek at a received share item", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "low"}}, capability.RiskLow, false),
		makeIOSTool(rt, "share", "receive.dismiss", "Dismiss Received Item", "Dismiss a received share item", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "share", "staging.cleanup", "Cleanup Share Staging", "Cleanup share staging area", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
		makeIOSTool(rt, "share", "limited.delete", "Delete Limited Share Item", "Delete a limited share item", genericInput, genericOutput, []capability.PermissionRequirement{{Capability: "ios.share.send", Risk: "medium"}}, capability.RiskMedium, true),
	}
	return defs
}

func makeIOSTool(rt capability.RuntimeBinding, module, operation, name, description string, inputSchema, outputSchema json.RawMessage, permissions []capability.PermissionRequirement, riskLevel capability.RiskLevel, hasSideEffects bool) capability.ToolDefinition {
	rt.HandlerName = operation
	return capability.ToolDefinition{
		ID:          "builtin:ios_native:" + module + "." + operation,
		ModelName:   "ios_native__" + module + "__" + replaceDots(operation),
		Source:      capability.ToolSourceBuiltin,
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		OutputSchema: outputSchema,
		Permissions: permissions,
		RiskLevel:   riskLevel,
		HasSideEffects: hasSideEffects,
		Enabled:     true,
		Runtime:     rt,
	}
}

func replaceDots(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			result = append(result, '_')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
