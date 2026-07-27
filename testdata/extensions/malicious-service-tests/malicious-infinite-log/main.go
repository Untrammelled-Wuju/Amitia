package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	maxLines := 0
	if v := os.Getenv("AMITIA_LOG_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxLines = n
		}
	}
	intervalMs := 5
	if v := os.Getenv("AMITIA_LOG_INTERVAL_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			intervalMs = n
		}
	}
	fmt.Fprintf(os.Stderr, "malicious-infinite-log: starting unlimited log spam (max=%d interval=%dms)\n", maxLines, intervalMs)
	i := 0
	for {
		fmt.Fprintf(os.Stderr, "spam-%d %s pid=%d line-noise-%d-filler-filler-filler-filler-filler-filler-filler-filler-filler\n", i, time.Now().UTC().Format(time.RFC3339Nano), os.Getpid(), i)
		i++
		if maxLines > 0 && i >= maxLines {
			break
		}
		if intervalMs > 0 {
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
		}
	}
	os.Exit(0)
}
