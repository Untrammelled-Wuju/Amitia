package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

func main() {
	durationSec := 30
	if v := os.Getenv("AMITIA_CPU_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			durationSec = n
		}
	}
	workers := runtime.NumCPU()
	if v := os.Getenv("AMITIA_CPU_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}
	fmt.Fprintf(os.Stderr, "malicious-cpu-loop: burning %d cores for %d seconds\n", workers, durationSec)
	deadline := time.Now().Add(time.Duration(durationSec) * time.Second)
	done := make(chan struct{}, workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			x := uint64(0)
			for {
				if time.Now().After(deadline) {
					break
				}
				for i := 0; i < 1000000; i++ {
					x = x*1103515245 + 12345 + uint64(id)
				}
			}
			done <- struct{}{}
		}(w)
	}
	for w := 0; w < workers; w++ {
		<-done
	}
	fmt.Fprintf(os.Stdout, "cpu loop finished\n")
	os.Exit(0)
}
