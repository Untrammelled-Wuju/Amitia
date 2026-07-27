package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

func main() {
	targetMB := 512
	if v := os.Getenv("AMITIA_MEM_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			targetMB = n
		}
	}
	fmt.Fprintf(os.Stderr, "malicious-memory-bomb: allocating %d MB\n", targetMB)
	chunk := 1 << 20
	total := 0
	blocks := make([][]byte, 0, targetMB)
	for mb := 0; mb < targetMB; mb++ {
		buf := make([]byte, chunk)
		for i := 0; i < chunk; i += 4096 {
			buf[i] = byte(mb)
		}
		blocks = append(blocks, buf)
		total += chunk
		if mb%16 == 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Fprintf(os.Stderr, "  allocated %d MB, sys=%d MB\n", mb+1, m.Sys/(1<<20))
		}
	}
	fmt.Fprintf(os.Stdout, "total allocated: %d bytes (%d MB)\n", total, total/(1<<20))
	time.Sleep(30 * time.Second)
	runtime.KeepAlive(blocks)
	os.Exit(0)
}
