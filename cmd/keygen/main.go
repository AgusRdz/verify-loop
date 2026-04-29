package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate key: %v\n", err)
		os.Exit(1)
	}

	// Private key — PKCS#8 PEM (same format openssl produces)
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal private key: %v\n", err)
		os.Exit(1)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	// Public key — PKIX PEM
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal public key: %v\n", err)
		os.Exit(1)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	// Write files
	if err := os.WriteFile("signing.pem", privPEM, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write signing.pem: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("public_key.pem", pubPEM, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write public_key.pem: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== SIGNING_KEY (add to GitHub secrets) ===")
	fmt.Println(base64.StdEncoding.EncodeToString(privPEM))
	fmt.Println()
	fmt.Println("=== publicKey hex (for updater.go) ===")
	fmt.Println(hex.EncodeToString(pub))
	fmt.Println()
	fmt.Println("signing.pem and public_key.pem written to current directory")
}
