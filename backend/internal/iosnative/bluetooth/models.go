package bluetooth

type AuthorizationStatus string

const (
	AuthNotDetermined AuthorizationStatus = "not_determined"
	AuthAllowed       AuthorizationStatus = "allowed"
	AuthDenied        AuthorizationStatus = "denied"
	AuthRestricted    AuthorizationStatus = "restricted"
)

type CentralState string

const (
	StateUnknown     CentralState = "unknown"
	StateResetting   CentralState = "resetting"
	StateUnsupported CentralState = "unsupported"
	StateUnauthorized CentralState = "unauthorized"
	StatePoweredOff  CentralState = "powered_off"
	StatePoweredOn   CentralState = "powered_on"
)

type ConnectionState string

const (
	ConnDisconnected  ConnectionState = "disconnected"
	ConnConnecting    ConnectionState = "connecting"
	ConnConnected     ConnectionState = "connected"
	ConnDisconnecting ConnectionState = "disconnecting"
	ConnReconnecting  ConnectionState = "reconnecting"
	ConnFailed        ConnectionState = "failed"
)

type BluetoothStatus struct {
	Supported                    bool                `json:"supported"`
	EnabledByUser                bool                `json:"enabledByUser"`
	Authorization                AuthorizationStatus `json:"authorization"`
	State                        CentralState        `json:"state"`
	PoweredOn                    bool                `json:"poweredOn"`
	IsScanning                   bool                `json:"isScanning"`
	ConnectedCount               int                 `json:"connectedCount"`
	CentralBackgroundEnabled     bool                `json:"centralBackgroundEnabled"`
	PeripheralRoleEnabled        bool                `json:"peripheralRoleEnabled"`
	PeripheralBackgroundEnabled  bool                `json:"peripheralBackgroundEnabled"`
	StateRestorationEnabled      bool                `json:"stateRestorationEnabled"`
	Generation                   uint64              `json:"generation"`
	Reason                       string              `json:"reason,omitempty"`
}

type BluetoothScanRequest struct {
	ServiceUUIDs            []string `json:"serviceUuids,omitempty"`
	DurationMs              int      `json:"durationMs"`
	AllowDuplicates         bool     `json:"allowDuplicates"`
	NamePrefix              string   `json:"namePrefix,omitempty"`
	RSSIMin                 *int     `json:"rssiMin,omitempty"`
	IncludeManufacturerData bool     `json:"includeManufacturerData,omitempty"`
	MaxResults              int      `json:"maxResults"`
}

type BluetoothBinarySummary struct {
	Length      int    `json:"length"`
	ContentHash string `json:"contentHash,omitempty"`
	Base64      string `json:"base64,omitempty"`
}

type BluetoothScanResult struct {
	ScanID                string                 `json:"scanId"`
	PeripheralID          string                 `json:"peripheralId"`
	Name                  string                 `json:"name,omitempty"`
	RSSI                  int                    `json:"rssi"`
	Connectable           *bool                  `json:"connectable,omitempty"`
	ServiceUUIDs          []string               `json:"serviceUuids,omitempty"`
	SolicitedServiceUUIDs []string               `json:"solicitedServiceUuids,omitempty"`
	OverflowServiceUUIDs  []string               `json:"overflowServiceUuids,omitempty"`
	ManufacturerData      *BluetoothBinarySummary `json:"manufacturerData,omitempty"`
	ServiceData           map[string]BluetoothBinarySummary `json:"serviceData,omitempty"`
	TxPower               *int                   `json:"txPower,omitempty"`
	SeenAt                int64                  `json:"seenAt"`
	Generation            uint64                 `json:"generation"`
}

type BluetoothPeripheralSession struct {
	PeripheralID       string          `json:"peripheralId"`
	Generation         uint64          `json:"generation"`
	State              ConnectionState `json:"state"`
	Name               string          `json:"name,omitempty"`
	ConnectedAt        int64           `json:"connectedAt,omitempty"`
	ServicesDiscovered bool            `json:"servicesDiscovered"`
	RSSI               *int            `json:"rssi,omitempty"`
	AutoReconnect      bool            `json:"autoReconnect"`
}

type BluetoothConnectRequest struct {
	PeripheralID         string   `json:"peripheralId"`
	TimeoutMs            int      `json:"timeoutMs"`
	AutoReconnect         bool     `json:"autoReconnect"`
	ExpectedServiceUUIDs []string `json:"expectedServiceUuids,omitempty"`
}

type BluetoothServiceInfo struct {
	PeripheralID         string   `json:"peripheralId"`
	ServiceUUID          string   `json:"serviceUuid"`
	ServiceRef           string   `json:"serviceRef"`
	Primary              bool     `json:"primary"`
	IncludedServiceUUIDs []string `json:"includedServiceUuids,omitempty"`
	Generation           uint64   `json:"generation"`
}

type BluetoothCharacteristicInfo struct {
	CharacteristicRef string   `json:"characteristicRef"`
	PeripheralID      string   `json:"peripheralId"`
	ServiceRef        string   `json:"serviceRef"`
	UUID              string   `json:"uuid"`
	Properties        []string `json:"properties"`
	IsNotifying       bool     `json:"isNotifying"`
	Value             *BluetoothValue `json:"value,omitempty"`
	Generation        uint64   `json:"generation"`
}

type BluetoothDescriptorInfo struct {
	DescriptorRef     string `json:"descriptorRef"`
	CharacteristicRef string `json:"characteristicRef"`
	UUID              string `json:"uuid"`
	Generation        uint64 `json:"generation"`
}

type BluetoothValue struct {
	Encoding    string `json:"encoding"`
	Base64      string `json:"base64,omitempty"`
	Hex         string `json:"hex,omitempty"`
	Length      int    `json:"length"`
	ContentHash string `json:"contentHash,omitempty"`
}

type BluetoothValueInput struct {
	Encoding string `json:"encoding"`
	Base64   string `json:"base64,omitempty"`
	Hex      string `json:"hex,omitempty"`
}

type BluetoothWriteRequest struct {
	CharacteristicRef string             `json:"characteristicRef"`
	Value             BluetoothValueInput `json:"value"`
	Mode              string             `json:"mode"`
	TimeoutMs         int                `json:"timeoutMs"`
}

type BluetoothCharacteristicEvent struct {
	PeripheralID        string        `json:"peripheralId"`
	CharacteristicRef   string        `json:"characteristicRef"`
	ServiceUUID         string        `json:"serviceUuid"`
	CharacteristicUUID  string        `json:"characteristicUuid"`
	Value               BluetoothValue `json:"value"`
	ObservedAt          int64         `json:"observedAt"`
	Generation          uint64        `json:"generation"`
}

type BluetoothPublishedService struct {
	UUID            string                              `json:"uuid"`
	Primary         bool                                `json:"primary"`
	Characteristics []BluetoothPublishedCharacteristic   `json:"characteristics"`
}

type BluetoothPublishedCharacteristic struct {
	UUID        string   `json:"uuid"`
	Properties  []string `json:"properties"`
	Permissions []string `json:"permissions"`
	InitialValue []byte  `json:"initialValue,omitempty"`
	MaxLength   int      `json:"maxLength"`
}

type BluetoothPeripheralRoleStatus struct {
	Enabled    bool         `json:"enabled"`
	State      CentralState `json:"state"`
	Advertising bool        `json:"advertising"`
	Services   int          `json:"services"`
	Generation uint64       `json:"generation"`
}
