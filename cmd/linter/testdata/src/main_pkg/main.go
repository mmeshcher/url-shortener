package main

import (
	"log"
	"os"
)

func main() {
	os.Exit(0)      // Allowed in main.main
	log.Fatal("ok") // Allowed in main.main
	panic("fail")   // want "usage of panic is prohibited"
}

func other() {
	os.Exit(1)      // want "usage of os.Exit is prohibited outside of main.main"
	log.Fatal("no") // want "usage of log.Fatal is prohibited outside of main.main"
}
