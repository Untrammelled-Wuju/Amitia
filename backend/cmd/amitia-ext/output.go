package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const (
	ExitSuccess  = 0
	ExitFailure  = 1
	ExitConfig   = 2
	ExitEnv      = 3
	ExitSig      = 4
	ExitInternal = 7
)

const CLIVersion = "0.1.0"

type Result struct {
	OK       bool        `json:"ok"`
	Message  string      `json:"message,omitempty"`
	Data     any         `json:"data,omitempty"`
	Errors   []string    `json:"errors,omitempty"`
	Warnings []string    `json:"warnings,omitempty"`
}

type Output struct {
	jsonMode bool
}

func newOutput(jsonMode bool) *Output {
	return &Output{jsonMode: jsonMode}
}

func (o *Output) emit(r Result) {
	if o.jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(os.Stderr, "内部错误: JSON编码失败: %v\n", err)
			os.Exit(ExitInternal)
		}
		return
	}
	if r.Message != "" {
		fmt.Println(r.Message)
	}
	for _, w := range r.Warnings {
		fmt.Printf("  警告: %s\n", w)
	}
	for _, e := range r.Errors {
		fmt.Printf("  错误: %s\n", e)
	}
	if r.Data != nil {
		printData(r.Data, 1)
	}
}

func (o *Output) fail(code int, msg string) {
	o.emit(Result{OK: false, Errors: []string{msg}})
	os.Exit(code)
}

func (o *Output) info(msg string) {
	if !o.jsonMode {
		fmt.Println(msg)
	}
}

func (o *Output) infof(format string, args ...any) {
	if !o.jsonMode {
		fmt.Printf(format, args...)
	}
}

func printData(data any, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}
	switch v := data.(type) {
	case string:
		fmt.Printf("%s%s\n", prefix, v)
	case []string:
		for _, s := range v {
			fmt.Printf("%s%s\n", prefix, s)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch child := v[k].(type) {
			case map[string]any:
				fmt.Printf("%s%s:\n", prefix, k)
				printData(child, indent+1)
			case []string:
				fmt.Printf("%s%s:\n", prefix, k)
				printData(child, indent+1)
			default:
				fmt.Printf("%s%s: %v\n", prefix, k, v[k])
			}
		}
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent(prefix, "  ")
		_ = enc.Encode(v)
	}
}
