package runtimeendpoint

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const SchemaVersion = 1

type ServiceEndpoint struct {
	HTTPBaseURL string `json:"httpBaseURL,omitempty"`
	GRPCBaseURL string `json:"grpcBaseURL,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	GRPCPort    int    `json:"grpcPort,omitempty"`
}

type Endpoint struct {
	SchemaVersion        int                        `json:"schemaVersion"`
	Version              string                     `json:"version,omitempty"`
	InstanceID           string                     `json:"instanceId,omitempty"`
	PID                  int                        `json:"pid"`
	StartedAt            string                     `json:"startedAt"`
	Host                 string                     `json:"host"`
	Port                 int                        `json:"port"`
	HTTPBaseURL          string                     `json:"httpBaseURL"`
	WebSocketBaseURL     string                     `json:"websocketBaseURL"`
	CompatibilityBaseURL string                     `json:"compatibilityBaseURL,omitempty"`
	Services             map[string]ServiceEndpoint `json:"services,omitempty"`
}

func FilePath(runDir string) string {
	return filepath.Join(runDir, "endpoint.json")
}

func New(version, instanceID, host string, port int) Endpoint {
	httpBaseURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	return Endpoint{
		SchemaVersion:    SchemaVersion,
		Version:          version,
		InstanceID:       instanceID,
		PID:              os.Getpid(),
		StartedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		Host:             host,
		Port:             port,
		HTTPBaseURL:      httpBaseURL,
		WebSocketBaseURL: toWebSocket(httpBaseURL),
		Services:         make(map[string]ServiceEndpoint),
	}
}

func Write(runDir string, endpoint Endpoint) error {
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(endpoint, "", "  ")
	if err != nil {
		return err
	}
	path := FilePath(runDir)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, append(data, '\n'), 0600); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(temp, path)
}

func Read(runDir string) (Endpoint, error) {
	data, err := os.ReadFile(FilePath(runDir))
	if err != nil {
		return Endpoint{}, err
	}
	var endpoint Endpoint
	if err := json.Unmarshal(data, &endpoint); err != nil {
		return Endpoint{}, err
	}
	return endpoint, nil
}

func toWebSocket(httpBaseURL string) string {
	if len(httpBaseURL) >= 5 && httpBaseURL[:5] == "https" {
		return "wss" + httpBaseURL[5:]
	}
	if len(httpBaseURL) >= 4 && httpBaseURL[:4] == "http" {
		return "ws" + httpBaseURL[4:]
	}
	return httpBaseURL
}
