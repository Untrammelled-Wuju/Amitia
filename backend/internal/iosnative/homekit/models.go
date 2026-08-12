package homekit

type InitializationState string

const (
	StateNotInitialized  InitializationState = "not_initialized"
	StateLoading         InitializationState = "loading"
	StateReady           InitializationState = "ready"
	StateUnauthorized    InitializationState = "unauthorized"
	StateFailed          InitializationState = "failed"
)

type AuthorizationStatus string

const (
	AuthNotDetermined AuthorizationStatus = "not_determined"
	AuthAuthorized    AuthorizationStatus = "authorized"
	AuthDenied        AuthorizationStatus = "denied"
	AuthRestricted    AuthorizationStatus = "restricted"
)

type HomeKitStatus struct {
	Supported bool `json:"supported"`

	EnabledByUser bool `json:"enabledByUser"`

	Initialized         bool `json:"initialized"`
	InitialLoadCompleted bool `json:"initialLoadCompleted"`

	Authorization string `json:"authorization"`

	HomeCount int `json:"homeCount"`

	CanRead    bool `json:"canRead"`
	CanControl bool `json:"canControl"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type HomeInfo struct {
	HomeID string `json:"homeId"`

	Name string `json:"name"`

	IsPrimary bool `json:"isPrimary"`

	RoomCount      int `json:"roomCount"`
	AccessoryCount int `json:"accessoryCount"`

	ActionSetCount int `json:"actionSetCount"`
	TriggerCount   int `json:"triggerCount"`
}

type HomeRoomInfo struct {
	RoomID string `json:"roomId"`

	HomeID string `json:"homeId"`

	Name string `json:"name"`

	IsEntireHome bool `json:"isEntireHome"`

	AccessoryCount int `json:"accessoryCount"`
}

type HomeZoneInfo struct {
	ZoneID string `json:"zoneId"`

	HomeID string `json:"homeId"`

	Name string `json:"name"`

	RoomIDs []string `json:"roomIds"`
}

type HomeAccessoryInfo struct {
	AccessoryID string `json:"accessoryId"`

	HomeID string `json:"homeId"`
	RoomID string `json:"roomId,omitempty"`

	Name string `json:"name"`

	Category string `json:"category,omitempty"`

	Manufacturer     string `json:"manufacturer,omitempty"`
	Model            string `json:"model,omitempty"`
	FirmwareVersion  string `json:"firmwareVersion,omitempty"`

	Reachable bool `json:"reachable"`

	ServiceCount int `json:"serviceCount"`

	Blocked bool `json:"blocked,omitempty"`
}

type HomeServiceInfo struct {
	ServiceID string `json:"serviceId"`

	AccessoryID string `json:"accessoryId"`

	Name string `json:"name"`

	ServiceType string `json:"serviceType"`

	Primary bool `json:"primary"`

	CharacteristicCount int `json:"characteristicCount"`
}

type HomeCharacteristicValue struct {
	Type string `json:"type"`

	Bool     *bool    `json:"bool,omitempty"`
	Integer  *int64   `json:"integer,omitempty"`
	Float    *float64 `json:"float,omitempty"`
	String   *string  `json:"string,omitempty"`

	DataBase64 string `json:"dataBase64,omitempty"`
}

type HomeCharacteristicInfo struct {
	CharacteristicID string `json:"characteristicId"`

	ServiceID    string `json:"serviceId"`
	AccessoryID  string `json:"accessoryId"`

	Type string `json:"type"`

	Name string `json:"name,omitempty"`

	Readable        bool `json:"readable"`
	Writable        bool `json:"writable"`
	SupportsEvents  bool `json:"supportsEvents"`

	Value *HomeCharacteristicValue `json:"value,omitempty"`

	Unit string `json:"unit,omitempty"`

	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	Step    *float64 `json:"step,omitempty"`

	ValidValues []HomeCharacteristicValue `json:"validValues,omitempty"`

	UpdatedAt int64 `json:"updatedAt,omitempty"`
}

type CharacteristicWriteResult struct {
	CharacteristicID string `json:"characteristicId"`

	RequestedValue HomeCharacteristicValue `json:"requestedValue"`

	Accepted bool `json:"accepted"`

	ReadBack *HomeCharacteristicValue `json:"readBack,omitempty"`

	Pending bool `json:"pending,omitempty"`
}

type HomeSceneAction struct {
	AccessoryID       string `json:"accessoryId"`
	ServiceID         string `json:"serviceId"`
	CharacteristicID  string `json:"characteristicId"`

	CharacteristicType string `json:"characteristicType"`

	TargetValue HomeCharacteristicValue `json:"targetValue"`

	Risk string `json:"risk"`
}

type HomeSceneInfo struct {
	SceneID string `json:"sceneId"`

	HomeID string `json:"homeId"`

	Name string `json:"name"`

	ActionCount int `json:"actionCount"`

	Builtin bool `json:"builtin,omitempty"`

	MaxRisk string `json:"maxRisk,omitempty"`
}

type HomeSceneDetail struct {
	HomeSceneInfo

	Actions []HomeSceneAction `json:"actions"`
}

type HomeAutomationInfo struct {
	AutomationID string `json:"automationId"`

	HomeID string `json:"homeId"`

	Name string `json:"name"`

	Type string `json:"type"`

	Enabled bool `json:"enabled"`

	ActionSetIDs []string `json:"actionSetIds,omitempty"`

	EventCount int `json:"eventCount,omitempty"`

	Conditions string `json:"conditions,omitempty"`
)

type HomeKitCharacteristicEvent struct {
	HomeID            string `json:"homeId"`
	AccessoryID       string `json:"accessoryId"`
	ServiceID         string `json:"serviceId"`
	CharacteristicID  string `json:"characteristicId"`

	Type string `json:"type"`

	Value HomeCharacteristicValue `json:"value"`

	ObservedAt int64 `json:"observedAt"`
)

type CharacteristicEventAutomationInput struct {
	AccessoryID      string `json:"accessoryId"`
	ServiceID        string `json:"serviceId"`
	CharacteristicID string `json:"characteristicId"`

	TargetValue HomeCharacteristicValue `json:"targetValue,omitempty"`
}

type CalendarEventAutomationInput struct {
	FireAt        string  `json:"fireAt"`
	Recurrence    string  `json:"recurrence,omitempty"`
	TimezoneOffset *int    `json:"timezoneOffset,omitempty"`
}

type PresenceEventAutomationInput struct {
	Event     string `json:"event"`
	UserScope string `json:"userScope"`
}

type CreateAutomationInput struct {
	HomeID      string `json:"homeId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ActionSetID string `json:"actionSetId,omitempty"`

	CharacteristicEvent *CharacteristicEventAutomationInput `json:"characteristicEvent,omitempty"`
	CalendarEvent       *CalendarEventAutomationInput       `json:"calendarEvent,omitempty"`
	PresenceEvent       *PresenceEventAutomationInput       `json:"presenceEvent,omitempty"`
}

type UpdateAutomationInput struct {
	AutomationID string `json:"automationId"`
	Name         *string `json:"name,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

type AccessorySetupRequest struct {
	HomeID     string `json:"homeId,omitempty"`
	RoomID     string `json:"roomId,omitempty"`
	SetupCode  string `json:"setupCode,omitempty"`
}

type AccessorySetupResult struct {
	Status    string `json:"status"`
	AccessoryID string `json:"accessoryId,omitempty"`
	Error     string `json:"error,omitempty"`
}
