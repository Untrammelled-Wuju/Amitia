package runtimeport

import (
	"fmt"
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
		_ = listener.Close()
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
			_ = first.Close()
			continue
		}
		second, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", secondPort))
		if err != nil {
			_ = first.Close()
			continue
		}
		_ = second.Close()
		_ = first.Close()
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
