package main

import (
	"github.com/example/ecoli-digital-twin-vslice/internal/api"
	"log"
	"os"
)

func main() {
	addr := os.Getenv("ECOLI_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	root := os.Getenv("ECOLI_ROOT")
	if root == "" {
		root = "."
	}
	log.Printf("E. coli twin API and web UI bound to %s", addr)
	log.Fatal(api.Listen(addr, root))
}
