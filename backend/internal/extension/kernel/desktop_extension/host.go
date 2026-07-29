package desktop_extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type DesktopSlotID string

const (
	SlotDesktopCommand         DesktopSlotID = "desktop.command"
	SlotDesktopAppMenu         DesktopSlotID = "desktop.application_menu.item"
	SlotDesktopContextMenu     DesktopSlotID = "desktop.context_menu.item"
	SlotDesktopTray            DesktopSlotID = "desktop.tray.item"
	SlotDesktopShortcut        DesktopSlotID = "desktop.shortcut"
	SlotDesktopNotification    DesktopSlotID = "desktop.notification.action"
	SlotDesktopWindowPage      DesktopSlotID = "desktop.window.page"
	SlotDesktopProtocolHandler DesktopSlotID = "desktop.protocol.handler"
	SlotDesktopFileOpen        DesktopSlotID = "desktop.file_open.action"
	SlotDesktopStatus          DesktopSlotID = "desktop.status.item"
)

type LocalizedText struct {
	Default       string            `json:"default"`
	Translations  map[string]string `json:"translations,omitempty"`
}

type DesktopCommandSpec struct {
	CommandID    string          `json:"commandId"`
	Title        LocalizedText   `json:"title"`
	Description  LocalizedText   `json:"description"`
	Icon         string          `json:"icon,omitempty"`
	Action       UIActionTarget  `json:"action"`
	Availability UIVisibilityRule `json:"availability"`
	RiskLevel    string          `json:"riskLevel,omitempty"`
}

type UIActionTarget struct {
	Type  string          `json:"type"`
	Input json.RawMessage `json:"input,omitempty"`
}

type UIVisibilityRule struct {
	RequiredContext []string `json:"requiredContext,omitempty"`
	Platforms       []string `json:"platforms,omitempty"`
	Conditions      []string `json:"conditions,omitempty"`
}

type DesktopMenuItemSpec struct {
	MenuID      string   `json:"menuId"`
	ParentSlot  string   `json:"parentSlot"`
	CommandID   string   `json:"commandId"`
	Group       string   `json:"group,omitempty"`
	Order       int      `json:"order"`
	Platforms   []string `json:"platforms,omitempty"`
	Separator   string   `json:"separator,omitempty"`
}

type DesktopTrayItemSpec struct {
	TrayID       string        `json:"trayId"`
	CommandID    string        `json:"commandId"`
	Title        LocalizedText `json:"title"`
	Icon         string        `json:"icon,omitempty"`
	Group        string        `json:"group,omitempty"`
	Order        int           `json:"order"`
	StatusText   string        `json:"statusText,omitempty"`
	UpdateRate   int           `json:"updateRatePerMinute,omitempty"`
}

type DesktopShortcutSpec struct {
	ShortcutID   string `json:"shortcutId"`
	CommandID    string `json:"commandId"`
	Accelerator  string `json:"accelerator"`
	Scope        string `json:"scope"`
	Global       bool   `json:"global"`
	Description  string `json:"description,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
}

type DesktopNotificationSpec struct {
	NotificationID string         `json:"notificationId"`
	Title          LocalizedText  `json:"title"`
	Body           LocalizedText  `json:"body"`
	Icon           string         `json:"icon,omitempty"`
	Action         UIActionTarget `json:"action"`
	Scope          string         `json:"scope"`
	MaxFrequency   int            `json:"maxFrequencyPerHour"`
}

type DesktopWindowPageSpec struct {
	PageID      string     `json:"pageId"`
	WindowRole  string     `json:"windowRole"`
	DefaultSize WindowSize `json:"defaultSize"`
	MinSize     WindowSize `json:"minSize"`
	MaxSize     WindowSize `json:"maxSize"`
	Resizable   bool       `json:"resizable"`
	Singleton   bool       `json:"singleton"`
	AlwaysOnTop bool       `json:"alwaysOnTop"`
}

type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type DesktopProtocolHandlerSpec struct {
	HandlerID  string   `json:"handlerId"`
	PathPrefix string   `json:"pathPrefix"`
	CommandID  string   `json:"commandId"`
	Platforms  []string `json:"platforms,omitempty"`
}

type DesktopFileOpenSpec struct {
	ActionID    string   `json:"actionId"`
	FileTypes   []string `json:"fileTypes"`
	MaxSizeBytes int64   `json:"maxSizeBytes"`
	CommandID   string   `json:"commandId"`
}

type DesktopContributionDefinition struct {
	ExtensionID      string
	ModuleID         string
	Generation       int64
	Command          *DesktopCommandSpec
	MenuItem         *DesktopMenuItemSpec
	TrayItem         *DesktopTrayItemSpec
	Shortcut         *DesktopShortcutSpec
	Notification     *DesktopNotificationSpec
	WindowPage       *DesktopWindowPageSpec
	ProtocolHandler  *DesktopProtocolHandlerSpec
	FileOpen         *DesktopFileOpenSpec
}

type DesktopExtensionHost struct {
	mu              sync.RWMutex
	commands        map[string]*DesktopCommandSpec
	commandsByExt   map[string][]string
	menuItems       map[string]*DesktopMenuItemSpec
	trayItems       map[string]*DesktopTrayItemSpec
	shortcuts       map[string]*DesktopShortcutSpec
	notifications   map[string]*DesktopNotificationSpec
	windowPages     map[string]*DesktopWindowPageSpec
	protocolHandlers map[string]*DesktopProtocolHandlerSpec
	fileOpenActions map[string]*DesktopFileOpenSpec
	resolver        CommandResolver
	permission      PermissionChecker
	shortcutsByAccelerator map[string]string
	trayCountByExt  map[string]int
	maxTrayPerExt   int
}

type CommandResolver interface {
	ResolveCommand(ctx context.Context, commandID string, scope string) (*ResolveResult, error)
}

type ResolveResult struct {
	Success    bool
	Result     json.RawMessage
	Error      string
}

type PermissionChecker interface {
	Check(ctx context.Context, extensionID, permission string) (bool, error)
}

func NewDesktopExtensionHost() *DesktopExtensionHost {
	return &DesktopExtensionHost{
		commands:               make(map[string]*DesktopCommandSpec),
		commandsByExt:         make(map[string][]string),
		menuItems:             make(map[string]*DesktopMenuItemSpec),
		trayItems:             make(map[string]*DesktopTrayItemSpec),
		shortcuts:             make(map[string]*DesktopShortcutSpec),
		notifications:         make(map[string]*DesktopNotificationSpec),
		windowPages:           make(map[string]*DesktopWindowPageSpec),
		protocolHandlers:      make(map[string]*DesktopProtocolHandlerSpec),
		fileOpenActions:       make(map[string]*DesktopFileOpenSpec),
		shortcutsByAccelerator: make(map[string]string),
		trayCountByExt:        make(map[string]int),
		maxTrayPerExt:         5,
	}
}

func (h *DesktopExtensionHost) SetResolver(r CommandResolver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resolver = r
}

func (h *DesktopExtensionHost) SetPermissionChecker(p PermissionChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.permission = p
}

func (h *DesktopExtensionHost) Register(def *DesktopContributionDefinition) error {
	if def == nil || def.ExtensionID == "" || def.ModuleID == "" {
		return ErrInvalidDefinition
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if def.Command != nil {
		if err := h.registerCommand(def.ExtensionID, def.Command); err != nil {
			return err
		}
	}
	if def.MenuItem != nil {
		if err := h.registerMenuItem(def.ExtensionID, def.MenuItem); err != nil {
			return err
		}
	}
	if def.TrayItem != nil {
		if err := h.registerTrayItem(def.ExtensionID, def.TrayItem); err != nil {
			return err
		}
	}
	if def.Shortcut != nil {
		if err := h.registerShortcut(def.ExtensionID, def.Shortcut); err != nil {
			return err
		}
	}
	if def.Notification != nil {
		if err := h.registerNotification(def.ExtensionID, def.Notification); err != nil {
			return err
		}
	}
	if def.WindowPage != nil {
		if err := h.registerWindowPage(def.ExtensionID, def.WindowPage); err != nil {
			return err
		}
	}
	if def.ProtocolHandler != nil {
		if err := h.registerProtocolHandler(def.ExtensionID, def.ProtocolHandler); err != nil {
			return err
		}
	}
	if def.FileOpen != nil {
		if err := h.registerFileOpen(def.ExtensionID, def.FileOpen); err != nil {
			return err
		}
	}
	return nil
}

func (h *DesktopExtensionHost) registerCommand(extID string, spec *DesktopCommandSpec) error {
	if spec.CommandID == "" {
		return ErrInvalidCommand
	}
	if _, exists := h.commands[spec.CommandID]; exists {
		return fmt.Errorf("%w: %s", ErrCommandExists, spec.CommandID)
	}
	h.commands[spec.CommandID] = spec
	h.commandsByExt[extID] = append(h.commandsByExt[extID], spec.CommandID)
	return nil
}

func (h *DesktopExtensionHost) registerMenuItem(extID string, spec *DesktopMenuItemSpec) error {
	if spec.MenuID == "" || spec.CommandID == "" {
		return ErrInvalidMenuItem
	}
	if !isValidMenuSlot(spec.ParentSlot) {
		return fmt.Errorf("%w: %s", ErrInvalidMenuSlot, spec.ParentSlot)
	}
	if _, exists := h.commands[spec.CommandID]; !exists {
		return fmt.Errorf("%w: %s", ErrCommandNotFound, spec.CommandID)
	}
	if _, exists := h.menuItems[spec.MenuID]; exists {
		return fmt.Errorf("%w: %s", ErrMenuItemExists, spec.MenuID)
	}
	h.menuItems[spec.MenuID] = spec
	return nil
}

func (h *DesktopExtensionHost) registerTrayItem(extID string, spec *DesktopTrayItemSpec) error {
	if spec.TrayID == "" || spec.CommandID == "" {
		return ErrInvalidTrayItem
	}
	if h.trayCountByExt[extID] >= h.maxTrayPerExt {
		return ErrTooManyTrayItems
	}
	if _, exists := h.trayItems[spec.TrayID]; exists {
		return fmt.Errorf("%w: %s", ErrTrayItemExists, spec.TrayID)
	}
	h.trayItems[spec.TrayID] = spec
	h.trayCountByExt[extID]++
	return nil
}

func (h *DesktopExtensionHost) registerShortcut(extID string, spec *DesktopShortcutSpec) error {
	if spec.ShortcutID == "" || spec.CommandID == "" || spec.Accelerator == "" {
		return ErrInvalidShortcut
	}
	if !isValidAccelerator(spec.Accelerator) {
		return fmt.Errorf("%w: %s", ErrInvalidAccelerator, spec.Accelerator)
	}
	if spec.Global {
		for _, reserved := range reservedGlobalShortcuts {
			if reserved == spec.Accelerator {
				return fmt.Errorf("%w: %s", ErrReservedShortcut, spec.Accelerator)
			}
		}
	}
	if existingID, exists := h.shortcutsByAccelerator[spec.Accelerator]; exists {
		return fmt.Errorf("%w: accelerator %s already used by %s", ErrShortcutConflict, spec.Accelerator, existingID)
	}
	if _, exists := h.shortcuts[spec.ShortcutID]; exists {
		return fmt.Errorf("%w: %s", ErrShortcutExists, spec.ShortcutID)
	}
	h.shortcuts[spec.ShortcutID] = spec
	h.shortcutsByAccelerator[spec.Accelerator] = spec.ShortcutID
	return nil
}

func (h *DesktopExtensionHost) registerNotification(extID string, spec *DesktopNotificationSpec) error {
	if spec.NotificationID == "" {
		return ErrInvalidNotification
	}
	if spec.MaxFrequency <= 0 {
		spec.MaxFrequency = 10
	}
	if _, exists := h.notifications[spec.NotificationID]; exists {
		return fmt.Errorf("%w: %s", ErrNotificationExists, spec.NotificationID)
	}
	h.notifications[spec.NotificationID] = spec
	return nil
}

func (h *DesktopExtensionHost) registerWindowPage(extID string, spec *DesktopWindowPageSpec) error {
	if spec.PageID == "" {
		return ErrInvalidWindowPage
	}
	if spec.AlwaysOnTop {
		return ErrAlwaysOnTopDenied
	}
	if spec.DefaultSize.Width <= 0 || spec.DefaultSize.Height <= 0 {
		spec.DefaultSize = WindowSize{Width: 800, Height: 600}
	}
	if spec.MinSize.Width <= 0 || spec.MinSize.Height <= 0 {
		spec.MinSize = WindowSize{Width: 320, Height: 240}
	}
	if spec.MaxSize.Width <= 0 || spec.MaxSize.Height <= 0 {
		spec.MaxSize = WindowSize{Width: 4096, Height: 4096}
	}
	if _, exists := h.windowPages[spec.PageID]; exists {
		return fmt.Errorf("%w: %s", ErrWindowPageExists, spec.PageID)
	}
	h.windowPages[spec.PageID] = spec
	return nil
}

func (h *DesktopExtensionHost) registerProtocolHandler(extID string, spec *DesktopProtocolHandlerSpec) error {
	if spec.HandlerID == "" || spec.PathPrefix == "" || spec.CommandID == "" {
		return ErrInvalidProtocolHandler
	}
	if !strings.HasPrefix(spec.PathPrefix, "/") {
		return ErrInvalidPathPrefix
	}
	if _, exists := h.protocolHandlers[spec.HandlerID]; exists {
		return fmt.Errorf("%w: %s", ErrProtocolHandlerExists, spec.HandlerID)
	}
	h.protocolHandlers[spec.HandlerID] = spec
	return nil
}

func (h *DesktopExtensionHost) registerFileOpen(extID string, spec *DesktopFileOpenSpec) error {
	if spec.ActionID == "" || spec.CommandID == "" {
		return ErrInvalidFileOpen
	}
	if len(spec.FileTypes) == 0 {
		return ErrNoFileTypes
	}
	if spec.MaxSizeBytes <= 0 {
		spec.MaxSizeBytes = 100 * 1024 * 1024
	}
	if _, exists := h.fileOpenActions[spec.ActionID]; exists {
		return fmt.Errorf("%w: %s", ErrFileOpenExists, spec.ActionID)
	}
	h.fileOpenActions[spec.ActionID] = spec
	return nil
}

type TriggerCommandRequest struct {
	CommandID    string          `json:"commandId"`
	ExtensionID  string          `json:"extensionId"`
	Scope        string          `json:"scope,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	Context      map[string]any  `json:"context,omitempty"`
}

func (h *DesktopExtensionHost) TriggerCommand(ctx context.Context, req TriggerCommandRequest) (*ResolveResult, error) {
	h.mu.RLock()
	command, exists := h.commands[req.CommandID]
	resolver := h.resolver
	perm := h.permission
	h.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrCommandNotFound, req.CommandID)
	}
	if perm != nil {
		permission := "desktop.command.register"
		ok, err := perm.Check(ctx, req.ExtensionID, permission)
		if err != nil {
			return &ResolveResult{Success: false, Error: err.Error()}, nil
		}
		if !ok {
			return &ResolveResult{Success: false, Error: "permission denied"}, nil
		}
	}
	if resolver == nil {
		return nil, ErrResolverUnavailable
	}
	_ = command
	return resolver.ResolveCommand(ctx, req.CommandID, req.Scope)
}

func (h *DesktopExtensionHost) UnregisterByExtension(extensionID string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, cmdID := range h.commandsByExt[extensionID] {
		delete(h.commands, cmdID)
		count++
	}
	delete(h.commandsByExt, extensionID)
	for id, item := range h.menuItems {
		_ = item
		_ = id
	}
	for id, tray := range h.trayItems {
		_ = tray
		_ = id
	}
	for id, sc := range h.shortcuts {
		_ = sc
		_ = id
	}
	h.shortcutsByAccelerator = make(map[string]string)
	delete(h.trayCountByExt, extensionID)
	return count, nil
}

type WindowOpenRequest struct {
	PageID    string `json:"pageId"`
	ExtensionID string `json:"extensionId"`
}

type WindowOpenResult struct {
	WindowID  string
	PageURL   string
	Size      WindowSize
	Singleton bool
}

func (h *DesktopExtensionHost) OpenWindow(req WindowOpenRequest) (*WindowOpenResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	spec, exists := h.windowPages[req.PageID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrWindowPageNotFound, req.PageID)
	}
	return &WindowOpenResult{
		WindowID:  fmt.Sprintf("wnd_%d", time.Now().UnixNano()),
		PageURL:   fmt.Sprintf("/extension/%s/page/%s", req.ExtensionID, req.PageID),
		Size:      spec.DefaultSize,
		Singleton: spec.Singleton,
	}, nil
}

var reservedGlobalShortcuts = []string{
	"CmdOrCtrl+N", "CmdOrCtrl+T", "CmdOrCtrl+W", "CmdOrCtrl+Q",
	"CmdOrCtrl+Shift+I", "CmdOrCtrl+R", "CmdOrCtrl+Shift+R",
	"Alt+F4", "CmdOrCtrl+Tab", "CmdOrCtrl+Shift+Tab",
}

func isValidAccelerator(a string) bool {
	if a == "" {
		return false
	}
	if strings.Contains(a, " ") {
		return false
	}
	return true
}

func isValidMenuSlot(slot string) bool {
	switch slot {
	case "extensions", "tools", "view", "help",
		"context.chat", "context.message", "context.extension":
		return true
	default:
		return false
	}
}

var (
	ErrInvalidDefinition        = errors.New("desktop_extension: invalid definition")
	ErrInvalidCommand           = errors.New("desktop_extension: invalid command")
	ErrCommandExists            = errors.New("desktop_extension: command exists")
	ErrCommandNotFound          = errors.New("desktop_extension: command not found")
	ErrInvalidMenuItem          = errors.New("desktop_extension: invalid menu item")
	ErrInvalidMenuSlot          = errors.New("desktop_extension: invalid menu slot")
	ErrMenuItemExists           = errors.New("desktop_extension: menu item exists")
	ErrInvalidTrayItem          = errors.New("desktop_extension: invalid tray item")
	ErrTooManyTrayItems         = errors.New("desktop_extension: too many tray items")
	ErrTrayItemExists           = errors.New("desktop_extension: tray item exists")
	ErrInvalidShortcut          = errors.New("desktop_extension: invalid shortcut")
	ErrInvalidAccelerator       = errors.New("desktop_extension: invalid accelerator")
	ErrReservedShortcut         = errors.New("desktop_extension: reserved shortcut")
	ErrShortcutConflict         = errors.New("desktop_extension: shortcut conflict")
	ErrShortcutExists           = errors.New("desktop_extension: shortcut exists")
	ErrInvalidNotification      = errors.New("desktop_extension: invalid notification")
	ErrNotificationExists       = errors.New("desktop_extension: notification exists")
	ErrInvalidWindowPage        = errors.New("desktop_extension: invalid window page")
	ErrAlwaysOnTopDenied        = errors.New("desktop_extension: always-on-top denied")
	ErrWindowPageExists         = errors.New("desktop_extension: window page exists")
	ErrWindowPageNotFound       = errors.New("desktop_extension: window page not found")
	ErrInvalidProtocolHandler   = errors.New("desktop_extension: invalid protocol handler")
	ErrInvalidPathPrefix        = errors.New("desktop_extension: invalid path prefix")
	ErrProtocolHandlerExists    = errors.New("desktop_extension: protocol handler exists")
	ErrInvalidFileOpen          = errors.New("desktop_extension: invalid file open action")
	ErrNoFileTypes              = errors.New("desktop_extension: no file types")
	ErrFileOpenExists           = errors.New("desktop_extension: file open action exists")
	ErrResolverUnavailable      = errors.New("desktop_extension: resolver unavailable")
)
