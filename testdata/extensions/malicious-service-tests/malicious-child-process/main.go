package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

func main() {
	count := 50
	if v := os.Getenv("AMITIA_MAL_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			count = n
		}
	}
	sleepSec := 30
	if v := os.Getenv("AMITIA_MAL_SLEEP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sleepSec = n
		}
	}
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve executable failed: %v\n", err)
		os.Exit(2)
	}
	var wg sync.WaitGroup
	pids := make([]int, 0, count)
	for i := 0; i < count; i++ {
		cmd := exec.Command(exePath, "--malicious-child")
		cmd.Env = append(os.Environ(), fmt.Sprintf("AMITIA_CHILD_SLEEP=%d", sleepSec))
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "spawn child %d failed: %v\n", i, err)
			continue
		}
		pids = append(pids, cmd.Process.Pid)
		wg.Add(1)
		go func(c *exec.Cmd) {
			defer wg.Done()
			_ = c.Wait()
		}(cmd)
	}
	fmt.Fprintf(os.Stderr, "malicious-child-process: spawned %d children pids=%v\n", len(pids), pids)
	time.Sleep(time.Duration(sleepSec) * time.Second)
	wg.Wait()
}

func init() {
	if len(os.Args) > 1 && os.Args[1] == "--malicious-child" {
		sleepSec := 30
		if v := os.Getenv("AMITIA_CHILD_SLEEP"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				sleepSec = n
			}
		}
		time.Sleep(time.Duration(sleepSec) * time.Second)
		os.Exit(0)
	}
}
