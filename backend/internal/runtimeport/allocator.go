package runtimeport

import (
	"fmt"
	amitialog "log"
	"net"
	"sync"
)

var (
	mu       sync.Mutex
	reserved = make(map[int]struct{})
	channels = make(map[string]string)
)

func AllocateLoopback() (int, error) {
	mu.Lock()
	defer mu.Unlock()
	for attempt := 0; attempt < 64; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := listener.Addr().(*net.TCPAddr).Port
		if err := listener.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if _, exists := reserved[port]; exists {
			continue
		}
		reserved[port] = struct{}{}
		return port, nil
	}
	return 0, fmt.Errorf("unable to allocate loopback port")
}

func AllocateLoopbackPair() (int, error) {
	mu.Lock()
	defer mu.Unlock()
	for attempt := 0; attempt < 64; attempt++ {
		first, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		firstPort := first.Addr().(*net.TCPAddr).Port
		secondPort := firstPort + 1
		if secondPort > 65535 {
			if err := first.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			continue
		}
		second, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", secondPort))
		if err != nil {
			if err := first.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			continue
		}
		if err := second.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := first.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if _, exists := reserved[firstPort]; exists {
			continue
		}
		if _, exists := reserved[secondPort]; exists {
			continue
		}
		reserved[firstPort] = struct{}{}
		reserved[secondPort] = struct{}{}
		return firstPort, nil
	}
	return 0, fmt.Errorf("unable to allocate loopback port pair")
}

func SetChannelEndpoint(channelID, baseURL string) {
	if channelID == "" || baseURL == "" {
		return
	}
	mu.Lock()
	channels[channelID] = baseURL
	mu.Unlock()
}

func ChannelEndpoint(channelID string) string {
	mu.Lock()
	defer mu.Unlock()
	return channels[channelID]
}
