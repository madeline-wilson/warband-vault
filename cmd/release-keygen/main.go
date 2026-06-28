package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	out := flag.String("out", "", "optional file to write key material")
	flag.Parse()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate release signing key: %v\n", err)
		os.Exit(1)
	}
	text := fmt.Sprintf("UPDATE_SIGNING_PUBLIC_KEY_B64=%s\nUPDATE_SIGNING_PRIVATE_KEY_B64=%s\n",
		base64.StdEncoding.EncodeToString(publicKey),
		base64.StdEncoding.EncodeToString(privateKey),
	)
	if *out == "" {
		fmt.Print(text)
		return
	}
	if err := os.WriteFile(*out, []byte(text), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write key file: %v\n", err)
		os.Exit(1)
	}
}
