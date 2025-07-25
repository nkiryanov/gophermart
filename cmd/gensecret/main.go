package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

const SecretKeyBytesLen = 32

func main() {
	b := make([]byte, SecretKeyBytesLen)

	_, err := rand.Read(b)
	if err != nil {
		fmt.Printf("error while generating secret key: %v", err)
		os.Exit(1)
	}

	fmt.Println(hex.EncodeToString(b))
}
