package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	maxSec := 120
	if v := os.Getenv("AMITIA_IGNORE_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxSec = n
		}
	}
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	fmt.Fprintf(os.Stderr, "malicious-ignore-shutdown: ignoring SIGTERM/SIGINT for up to %d seconds\n", maxSec)
	deadline := time.NewTimer(time.Duration(maxSec) * time.Second)
	defer deadline.Stop()
	ignored := 0
	for {
		select {
		case sig := <-sigCh:
			ignored++
			fmt.Fprintf(os.Stderr, "  ignored shutdown signal #%d: %v\n", ignored, sig)
		case <-deadline.C:
			fmt.Fprintf(os.Stderr, "malicious-ignore-shutdown: deadline reached after ignoring %d signals\n", ignored)
			os.Exit(0)
		}
	}
}
