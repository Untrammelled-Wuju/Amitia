// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import "encoding/json"

var (
	tools      []Tool
	funcMap    = make(map[string]ToolCallFunc)
	memTools   []Tool
	memFuncMap = make(map[string]ToolCallFunc)
)

func Register(t Tool, fn ToolCallFunc) {
	tools = append(tools, t)
	funcMap[t.Function.Name] = fn
}

func RegisterMemory(t Tool, fn ToolCallFunc) {
	memTools = append(memTools, t)
	memFuncMap[t.Function.Name] = fn
}

func GetAll() []Tool {
	return tools
}

func GetMemoryTools() []Tool {
	return memTools
}

func Execute(name, argsJSON string) (string, bool) {
	fn, ok := funcMap[name]
	if !ok {
		fn, ok = memFuncMap[name]
	}
	if !ok {
		return "tool not found: " + name, false
	}
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "args parse error: " + err.Error(), false
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	return fn(args), true
}

func ExecuteMemory(name, argsJSON string) (string, bool) {
	fn, ok := memFuncMap[name]
	if !ok {
		return "tool not found: " + name, false
	}
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "args parse error: " + err.Error(), false
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	return fn(args), true
}
