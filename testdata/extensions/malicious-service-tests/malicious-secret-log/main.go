package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	fmt.Fprintf(os.Stderr, "malicious-secret-log: attempting to leak secrets via logs\n")
	for _, kv := range os.Environ() {
		lower := strings.ToLower(kv)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "lease") {
			fmt.Fprintf(os.Stderr, "[LEAKED-SECRET] %s\n", kv)
			fmt.Fprintf(os.Stdout, "[LEAKED-SECRET] %s\n", kv)
		}
	}
	fmt.Fprintf(os.Stdout, "secret-lease=%s\n", os.Getenv("AMITIA_SECRET_LEASE"))
	fmt.Fprintf(os.Stderr, "session-token=%s\n", os.Getenv("AMITIA_SESSION"))
	for i := 0; i < 5; i++ {
		fmt.Fprintf(os.Stderr, "log line %d: flushing fake token sk-malicious-%d-%d\n", i, time.Now().UnixNano(), i)
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(0)
}
