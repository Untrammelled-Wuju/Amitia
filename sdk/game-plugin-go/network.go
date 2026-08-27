package sdk

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	MethodHostNetworkRequest          = "host.network.request"
	MethodHostNetworkTCPOpen          = "host.network.tcp.open"
	MethodHostNetworkTCPRead          = "host.network.tcp.read"
	MethodHostNetworkTCPWrite         = "host.network.tcp.write"
	MethodHostNetworkTCPClose         = "host.network.tcp.close"
	MethodHostNetworkUDPOpen          = "host.network.udp.open"
	MethodHostNetworkUDPReceive       = "host.network.udp.receive"
	MethodHostNetworkUDPSend          = "host.network.udp.send"
	MethodHostNetworkUDPClose         = "host.network.udp.close"
	MethodHostNetworkWebSocketOpen    = "host.network.websocket.open"
	MethodHostNetworkWebSocketReceive = "host.network.websocket.receive"
	MethodHostNetworkWebSocketSend    = "host.network.websocket.send"
	MethodHostNetworkWebSocketClose   = "host.network.websocket.close"
)

type NetworkRequestInput struct {
	Method           string            `json:"method,omitempty"`
	URL              string            `json:"url"`
	Headers          map[string]string `json:"headers,omitempty"`
	BodyBase64       string            `json:"bodyBase64,omitempty"`
	TimeoutMs        int               `json:"timeoutMs,omitempty"`
	MaxResponseBytes int64             `json:"maxResponseBytes,omitempty"`
}

type NetworkRequestOutput struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"bodyBase64"`
	FinalURL   string              `json:"finalUrl"`
}

type NetworkSocketOpenInput struct {
	Target    string `json:"target"`
	Port      int    `json:"port"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketOpenOutput struct {
	HandleID      string `json:"handleId"`
	Transport     string `json:"transport"`
	LocalAddress  string `json:"localAddress,omitempty"`
	RemoteAddress string `json:"remoteAddress,omitempty"`
}

type NetworkSocketReadInput struct {
	HandleID  string `json:"handleId"`
	MaxBytes  int    `json:"maxBytes,omitempty"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketReadOutput struct {
	DataBase64 string `json:"dataBase64"`
	BytesRead  int    `json:"bytesRead"`
	EOF        bool   `json:"eof,omitempty"`
}

type NetworkSocketWriteInput struct {
	HandleID   string `json:"handleId"`
	DataBase64 string `json:"dataBase64"`
	TimeoutMs  int    `json:"timeoutMs,omitempty"`
}

type NetworkSocketWriteOutput struct {
	BytesWritten int `json:"bytesWritten"`
}

type NetworkSocketCloseInput struct {
	HandleID string `json:"handleId"`
}

type NetworkSocketCloseOutput struct {
	Closed bool `json:"closed"`
}

type NetworkWebSocketOpenInput struct {
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Subprotocols []string          `json:"subprotocols,omitempty"`
	TimeoutMs    int               `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketOpenOutput struct {
	HandleID      string `json:"handleId"`
	Subprotocol   string `json:"subprotocol,omitempty"`
	RemoteAddress string `json:"remoteAddress,omitempty"`
}

type NetworkWebSocketSendInput struct {
	HandleID    string `json:"handleId"`
	MessageType string `json:"messageType,omitempty"`
	DataBase64  string `json:"dataBase64"`
	TimeoutMs   int    `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketSendOutput struct {
	BytesWritten int `json:"bytesWritten"`
}

type NetworkWebSocketReceiveInput struct {
	HandleID  string `json:"handleId"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkWebSocketReceiveOutput struct {
	MessageType string `json:"messageType"`
	DataBase64  string `json:"dataBase64"`
	BytesRead   int    `json:"bytesRead"`
}

func invokeTypedHostNetworkAPI(ctx context.Context, client *Client, method string, input any, output any, opts ...MessageOption) error {
	if client == nil {
		return fmt.Errorf("game plugin sdk: client is required")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return NewEncodeError("marshal %s input: %v", method, err)
	}
	result, err := client.InvokeHostAPI(ctx, HostInvokeInput{Method: method, Version: 1, Input: encoded}, opts...)
	if err != nil {
		return err
	}
	if result.Status != HostAPISuccess {
		return fmt.Errorf("game plugin sdk: %s returned host API status %q", method, result.Status)
	}
	if output == nil || len(result.Output) == 0 {
		return nil
	}
	if err := json.Unmarshal(result.Output, output); err != nil {
		return NewEncodeError("unmarshal %s output: %v", method, err)
	}
	return nil
}

func NetworkRequest(ctx context.Context, client *Client, input NetworkRequestInput, opts ...MessageOption) (NetworkRequestOutput, error) {
	var out NetworkRequestOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkRequest, input, &out, opts...)
	return out, err
}

func NetworkTCPOpen(ctx context.Context, client *Client, input NetworkSocketOpenInput, opts ...MessageOption) (NetworkSocketOpenOutput, error) {
	var out NetworkSocketOpenOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkTCPOpen, input, &out, opts...)
	return out, err
}

func NetworkTCPRead(ctx context.Context, client *Client, input NetworkSocketReadInput, opts ...MessageOption) (NetworkSocketReadOutput, error) {
	var out NetworkSocketReadOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkTCPRead, input, &out, opts...)
	return out, err
}

func NetworkTCPWrite(ctx context.Context, client *Client, input NetworkSocketWriteInput, opts ...MessageOption) (NetworkSocketWriteOutput, error) {
	var out NetworkSocketWriteOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkTCPWrite, input, &out, opts...)
	return out, err
}

func NetworkTCPClose(ctx context.Context, client *Client, input NetworkSocketCloseInput, opts ...MessageOption) (NetworkSocketCloseOutput, error) {
	var out NetworkSocketCloseOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkTCPClose, input, &out, opts...)
	return out, err
}

func NetworkUDPOpen(ctx context.Context, client *Client, input NetworkSocketOpenInput, opts ...MessageOption) (NetworkSocketOpenOutput, error) {
	var out NetworkSocketOpenOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkUDPOpen, input, &out, opts...)
	return out, err
}

func NetworkUDPReceive(ctx context.Context, client *Client, input NetworkSocketReadInput, opts ...MessageOption) (NetworkSocketReadOutput, error) {
	var out NetworkSocketReadOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkUDPReceive, input, &out, opts...)
	return out, err
}

func NetworkUDPSend(ctx context.Context, client *Client, input NetworkSocketWriteInput, opts ...MessageOption) (NetworkSocketWriteOutput, error) {
	var out NetworkSocketWriteOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkUDPSend, input, &out, opts...)
	return out, err
}

func NetworkUDPClose(ctx context.Context, client *Client, input NetworkSocketCloseInput, opts ...MessageOption) (NetworkSocketCloseOutput, error) {
	var out NetworkSocketCloseOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkUDPClose, input, &out, opts...)
	return out, err
}

func NetworkWebSocketOpen(ctx context.Context, client *Client, input NetworkWebSocketOpenInput, opts ...MessageOption) (NetworkWebSocketOpenOutput, error) {
	var out NetworkWebSocketOpenOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkWebSocketOpen, input, &out, opts...)
	return out, err
}

func NetworkWebSocketReceive(ctx context.Context, client *Client, input NetworkWebSocketReceiveInput, opts ...MessageOption) (NetworkWebSocketReceiveOutput, error) {
	var out NetworkWebSocketReceiveOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkWebSocketReceive, input, &out, opts...)
	return out, err
}

func NetworkWebSocketSend(ctx context.Context, client *Client, input NetworkWebSocketSendInput, opts ...MessageOption) (NetworkWebSocketSendOutput, error) {
	var out NetworkWebSocketSendOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkWebSocketSend, input, &out, opts...)
	return out, err
}

func NetworkWebSocketClose(ctx context.Context, client *Client, input NetworkSocketCloseInput, opts ...MessageOption) (NetworkSocketCloseOutput, error) {
	var out NetworkSocketCloseOutput
	err := invokeTypedHostNetworkAPI(ctx, client, MethodHostNetworkWebSocketClose, input, &out, opts...)
	return out, err
}
