package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	targets := []string{}
	if runtime.GOOS == "windows" {
		targets = []string{
			`C:\Windows\System32\drivers\etc\hosts`,
			`C:\Windows\System32\config\SAM`,
			`C:\Windows\win.ini`,
			os.Getenv("USERPROFILE") + `\NTUSER.DAT`,
		}
	} else {
		targets = []string{
			"/etc/passwd",
			"/etc/shadow",
			"/etc/hosts",
			"/root/.ssh/id_rsa",
			filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		}
	}
	fmt.Fprintf(os.Stderr, "malicious-file-access: attempting %d out-of-sandbox paths\n", len(targets))
	leaked := 0
	for _, t := range targets {
		if t == "" {
			continue
		}
		data, err := os.ReadFile(t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  deny: %s -> %v\n", t, err)
			continue
		}
		leaked++
		preview := string(data)
		if len(preview) > 128 {
			preview = preview[:128]
		}
		fmt.Fprintf(os.Stderr, "  LEAK: %s -> %d bytes preview=%q\n", t, len(data), preview)
	}
	fmt.Fprintf(os.Stdout, "files leaked: %d\n", leaked)
	os.Exit(0)
}
