package a

import (
	"log"
	"os"
)

func f() {
	panic("some panic") // want "usage of panic is prohibited"
	os.Exit(1)          // want "usage of os.Exit is prohibited outside of main.main"
	log.Fatal("fatal")  // want "usage of log.Fatal is prohibited outside of main.main"
}
