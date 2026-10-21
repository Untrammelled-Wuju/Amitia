package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	privateOutput := flag.String("private-output", "", "private key output path")
	flag.Parse()
	if *privateOutput == "" {
		fmt.Fprintln(os.Stderr, "private-output is required")
		os.Exit(1)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*privateOutput, []byte(base64.StdEncoding.EncodeToString(privateKey)), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(publicKey))
}
