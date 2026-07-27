package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"time"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "malicious-binary-replace: resolve executable failed: %v\n", err)
		os.Exit(2)
	}
	original, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "malicious-binary-replace: read own binary failed: %v\n", err)
		os.Exit(2)
	}
	origHash := sha256.Sum256(original)
	fmt.Fprintf(os.Stderr, "malicious-binary-replace: target=%s size=%d hash=%s\n", exePath, len(original), hex.EncodeToString(origHash[:]))
	tampered := make([]byte, len(original))
	copy(tampered, original)
	if len(tampered) > 64 {
		tampered[64] = tampered[64] ^ 0xFF
		tampered[65] = tampered[65] ^ 0xFF
	} else if len(tampered) > 0 {
		tampered[0] = tampered[0] ^ 0xFF
	}
	err = os.WriteFile(exePath, tampered, 0o755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "malicious-binary-replace: overwrite blocked by runtime: %v\n", err)
		os.Exit(3)
	}
	newHash := sha256.Sum256(tampered)
	fmt.Fprintf(os.Stderr, "malicious-binary-replace: SUCCESS overwrote own binary new_hash=%s\n", hex.EncodeToString(newHash[:]))
	fmt.Fprintf(os.Stdout, "binary replaced: %s\n", exePath)
	if runtime.GOOS == "windows" {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(exePath, original, 0o755)
		fmt.Fprintf(os.Stderr, "malicious-binary-replace: restored original (windows allows write while running)\n")
	}
	os.Exit(0)
}
