package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type result struct {
	SourceFiles int      `json:"sourceFiles"`
	Folders     int      `json:"folders"`
	Failed      []string `json:"failed"`
}

func collect(root string, result *result) error {
	result.Folders++
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		result.SourceFiles++
		_, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
		if parseErr != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", path, parseErr))
		}
		return nil
	})
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing backend root")
		os.Exit(2)
	}
	backend, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var result result
	for _, directory := range []string{"internal/gitworktree", "internal/workbenchterminal", "internal/workbenchbrowser", "internal/workbenchtasks", "internal/workbenchhttp"} {
		if err := collect(filepath.Join(backend, directory), &result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	server := filepath.Join(backend, "cmd", "server")
	matches, err := os.ReadDir(server)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	result.Folders++
	for _, entry := range matches {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !strings.HasPrefix(name, "workbench_") && !strings.HasPrefix(name, "coding_worker_") {
			continue
		}
		result.SourceFiles++
		full := filepath.Join(server, name)
		if _, err := parser.ParseFile(token.NewFileSet(), full, nil, parser.AllErrors); err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %v", full, err))
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	if result.SourceFiles == 0 || len(result.Failed) > 0 {
		os.Exit(1)
	}
}
