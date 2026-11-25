//go:build rsi_routee2e

package routee2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

type helperRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type helperResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *helperRPCError `json:"error,omitempty"`
}

type helperRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type helperInvokeParams struct {
	AdapterID       string `json:"adapterId"`
	ContractVersion int    `json:"contractVersion"`
	Request         struct {
		InvocationID string          `json:"invocationId"`
		Operation    string          `json:"operation"`
		Input        json.RawMessage `json:"input"`
	} `json:"request"`
}

type coreCallback struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Space  string          `json:"space"`
	Input  json.RawMessage `json:"input"`
}

type coreCallbackReply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func runPluginHelper() int {
	mode := strings.TrimSpace(os.Getenv(modeEnv))
	socket := strings.TrimSpace(os.Getenv(callbackEnv))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request helperRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			continue
		}
		response := helperResponse{JSONRPC: "2.0", ID: request.ID}
		if request.Method == "adapter.invoke" {
			output, err := helperInvoke(mode, socket, request.Params)
			if err != nil {
				response.Error = &helperRPCError{Code: 403, Message: err.Error()}
			} else {
				var invoke helperInvokeParams
				_ = json.Unmarshal(request.Params, &invoke)
				response.Result = map[string]any{
					"invocationId": invoke.Request.InvocationID,
					"output":       json.RawMessage(output),
				}
			}
		} else {
			response.Result = helperStatic(request.Method, request.Params)
		}
		if err := encoder.Encode(response); err != nil {
			return 1
		}
	}
	return 0
}

func helperStatic(method string, params json.RawMessage) interface{} {
	switch method {
	case "adapter.build":
		return map[string]any{"id": pluginProviderID, "handleId": "handle-1"}
	case "adapter.health":
		return map[string]any{"healthy": true}
	case "adapter.activate", "adapter.deactivate", "adapter.discard", "adapter.cancel":
		return struct{}{}
	default:
		_ = params
		return struct{}{}
	}
}

func helperInvoke(mode string, socket string, params json.RawMessage) (json.RawMessage, error) {
	var invoke helperInvokeParams
	if err := json.Unmarshal(params, &invoke); err != nil {
		return nil, fmt.Errorf("routee2e: decode invoke params: %w", err)
	}
	switch mode {
	case "crash-hang":
		fmt.Fprintln(os.Stderr, "READY")
		select {}
	case "deny":
		if _, err := callCore(socket, "memory.write", allowedSpace, invoke.Request.Input); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("routee2e: helper expected the write to be refused")
	case "cross-scope":
		if _, err := callCore(socket, "memory.search", "other-space", invoke.Request.Input); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("routee2e: helper expected the cross-scope read to be refused")
	default:
		return callCore(socket, "memory.search", allowedSpace, invoke.Request.Input)
	}
}

func callCore(socket string, method string, space string, input json.RawMessage) (json.RawMessage, error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("routee2e: dial core callback: %w", err)
	}
	defer conn.Close()
	request := coreCallback{ID: 1, Method: method, Space: space, Input: input}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return nil, fmt.Errorf("routee2e: send core callback: %w", err)
	}
	var reply coreCallbackReply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return nil, fmt.Errorf("routee2e: read core callback: %w", err)
	}
	if reply.Error != "" {
		return nil, fmt.Errorf("%s", reply.Error)
	}
	if len(reply.Result) == 0 {
		return nil, fmt.Errorf("routee2e: core callback returned no result")
	}
	return reply.Result, nil
}
