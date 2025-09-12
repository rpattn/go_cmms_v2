// genhash/main.go

//$env:SEED_PASSWORD='Super-Long-Temp-Password'; go run .\genhash.go

package main

import (
	"fmt"
	"log"
	"os"

	"yourapp/internal/auth"
)

type ArgonParams struct {
	Time    uint32
	Memory  uint32 // in KiB (e.g., 64<<10 for 64 MiB)
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

func main() {
	pw := os.Getenv("SEED_PASSWORD")
	if pw == "" {
		log.Fatal("set SEED_PASSWORD")
	}
	p := auth.ArgonParams{
		Time:    3,
		Memory:  64 << 10, // 64 MiB
		Threads: 1,
		SaltLen: 16,
		KeyLen:  32,
	}
	phc, err := auth.HashPassword(pw, p)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(phc)
}
