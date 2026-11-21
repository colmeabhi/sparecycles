// Command keygen prints a new Ed25519 key pair for task signing, as two .env lines.
//
// Put SC_SIGNING_KEY in the .env of every coordinator, and SC_SIGNING_PUBLIC_KEY
// in the .env of every volunteer. Never commit the private key.
package main

import (
	"fmt"
	"os"

	"github.com/colmeabhi/sparecycles/internal/security"
)

func main() {
	seed, public, err := security.GenerateKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("# Coordinators only. Keep this secret.")
	fmt.Printf("SC_SIGNING_KEY=%s\n", seed)
	fmt.Println("# Volunteers.")
	fmt.Printf("SC_SIGNING_PUBLIC_KEY=%s\n", public)
}
