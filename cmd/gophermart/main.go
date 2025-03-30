package main

import (
	"github.com/patric-chuzhbe/diploma/internal/app"
	"log"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
