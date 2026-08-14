package iosnative

import (
	"github.com/u-ai/backend/internal/iosnative/alarms"
	"github.com/u-ai/backend/internal/iosnative/background"
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
	"github.com/u-ai/backend/internal/nativebridge"
)

func NewCanonicalProvider(bridge nativebridge.Bridge) (*Provider, error) {
	provider := NewProvider(bridge)

	healthHandler := health.NewHealthHandler(bridge)
	for _, op := range health.Operations() {
		if err := provider.RegisterHandler(op, healthHandler); err != nil {
			return nil, err
		}
	}

	calendarHandler := calendar.NewCalendarHandler(bridge)
	for _, op := range calendar.Operations() {
		if err := provider.RegisterHandler(op, calendarHandler); err != nil {
			return nil, err
		}
	}

	remindersHandler := reminders.NewRemindersHandler(bridge)
	for _, op := range reminders.Operations() {
		if err := provider.RegisterHandler(op, remindersHandler); err != nil {
			return nil, err
		}
	}

	contactsHandler := contacts.NewContactsHandler(bridge)
	for _, op := range contacts.Operations() {
		if err := provider.RegisterHandler(op, contactsHandler); err != nil {
			return nil, err
		}
	}

	homekitHandler := homekit.NewHomeKitHandler(bridge)
	for _, op := range homekit.Operations() {
		if err := provider.RegisterHandler(op, homekitHandler); err != nil {
			return nil, err
		}
	}

	bluetoothHandler := bluetooth.NewBluetoothHandler(bridge)
	for _, op := range bluetooth.Operations() {
		if err := provider.RegisterHandler(op, bluetoothHandler); err != nil {
			return nil, err
		}
	}

	clipboardHandler := clipboard.NewClipboardHandler(bridge)
	for _, op := range clipboard.Operations() {
		if err := provider.RegisterHandler(op, clipboardHandler); err != nil {
			return nil, err
		}
	}

	mediaHandler := media.NewMediaHandler(bridge)
	for _, op := range media.Operations() {
		if err := provider.RegisterHandler(op, mediaHandler); err != nil {
			return nil, err
		}
	}

	alarmHandler := alarms.NewAlarmHandler(bridge)
	for _, op := range alarms.Operations() {
		if err := provider.RegisterHandler(op, alarmHandler); err != nil {
			return nil, err
		}
	}

	shareHandler := share.NewShareHandler(bridge)
	for _, op := range share.Operations() {
		if err := provider.RegisterHandler(op, shareHandler); err != nil {
			return nil, err
		}
	}

	shortcutsHandler := shortcuts.NewShortcutsHandler(bridge)
	for _, op := range shortcuts.Operations() {
		if err := provider.RegisterHandler(op, shortcutsHandler); err != nil {
			return nil, err
		}
	}

	backgroundHandler := background.NewBackgroundHandler(bridge)
	for _, op := range background.Operations() {
		if err := provider.RegisterHandler(op, backgroundHandler); err != nil {
			return nil, err
		}
	}

	return provider, nil
}
