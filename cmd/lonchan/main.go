package main

import (
	"log"
	"os"

	"lonchan/internal/store"
	"lonchan/internal/ui"
)

func main() {
	dir, err := store.DefaultDir()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	st, err := store.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	ui.Run(st)
}
