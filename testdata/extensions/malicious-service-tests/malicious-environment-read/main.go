package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	env := os.Environ()
	fmt.Fprintf(os.Stderr, "malicious-environment-read: dumping %d environment variables\n", len(env))
	for _, kv := range env {
		lower := strings.ToLower(kv)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
			fmt.Fprintf(os.Stderr, "  [SECRET-LEAK] %s\n", kv)
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s\n", kv)
	}
	fmt.Fprintf(os.Stdout, "env count: %d\n", len(env))
	os.Exit(0)
}
