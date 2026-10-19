package main

import (
	"log"

	"GoArchivist/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
