package platform

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func isTCPPortInUse(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForTCPPortFree(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !isTCPPortInUse(addr) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port still occupied: %s", addr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func writePidFile(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	file, err := os.OpenFile(pidPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = file.Close()
		_ = os.Remove(pidPath)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(pidPath)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(pidPath)
		return err
	}
	return nil
}

func killExistingServer(addr, dataDir string, readPid func(string) (int, error), removePid func(string) error) error {
	portInUse := isTCPPortInUse(addr)
	pid, err := readPid(dataDir)
	if err != nil {
		if !portInUse {
			return nil
		}
		return fmt.Errorf("port %s is occupied without a valid pid file: %w", addr, err)
	}
	if pid <= 0 {
		if !portInUse {
			_ = removePid(dataDir)
			return nil
		}
		return fmt.Errorf("port %s is occupied by invalid pid %d", addr, pid)
	}
	if pid == os.Getpid() {
		if portInUse {
			return fmt.Errorf("port %s is occupied by current process pid=%d", addr, pid)
		}
		return nil
	}

	currentImage, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	targetImage, err := processImagePath(pid)
	if err != nil {
		if !portInUse {
			_ = removePid(dataDir)
			return nil
		}
		return fmt.Errorf("resolve process %d executable: %w", pid, err)
	}
	if !sameProcessImage(currentImage, targetImage) {
		if !portInUse {
			_ = removePid(dataDir)
			return nil
		}
		return fmt.Errorf("port %s is occupied by pid=%d executable=%s", addr, pid, targetImage)
	}
	if err := terminateProcess(pid); err != nil {
		return fmt.Errorf("terminate process %d: %w", pid, err)
	}
	if err := waitForTCPPortFree(addr, 5*time.Second); err != nil {
		return err
	}
	_ = removePid(dataDir)
	return nil
}
