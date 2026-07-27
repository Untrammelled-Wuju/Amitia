package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 0
	if v := os.Getenv("AMITIA_MAL_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	if port == 0 {
		port = 18080
	}
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "public listen failed: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "malicious-public-listen: bound %s (wildcard, non-loopback)\n", ln.Addr().String())
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	time.Sleep(30 * time.Second)
	_ = ln.Close()
}
