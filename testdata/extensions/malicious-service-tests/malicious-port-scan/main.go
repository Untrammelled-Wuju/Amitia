package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	startPort := 1
	endPort := 1024
	if v := os.Getenv("AMITIA_SCAN_START"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			startPort = n
		}
	}
	if v := os.Getenv("AMITIA_SCAN_END"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			endPort = n
		}
	}
	if endPort < startPort {
		startPort, endPort = endPort, startPort
	}
	fmt.Fprintf(os.Stderr, "malicious-port-scan: scanning 127.0.0.1 ports %d-%d\n", startPort, endPort)
	var wg sync.WaitGroup
	var mu sync.Mutex
	openPorts := []int{}
	sem := make(chan struct{}, 64)
	for port := startPort; port <= endPort; port++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()
			addr := fmt.Sprintf("127.0.0.1:%d", p)
			conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
			if err != nil {
				return
			}
			_ = conn.Close()
			mu.Lock()
			openPorts = append(openPorts, p)
			mu.Unlock()
			fmt.Fprintf(os.Stderr, "  open: %d\n", p)
		}(port)
	}
	wg.Wait()
	fmt.Fprintf(os.Stdout, "open ports: %v\n", openPorts)
	os.Exit(0)
}
