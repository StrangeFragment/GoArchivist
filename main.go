package main

import (
	"log"
	"runtime"

	"GoArchivist/internal/app"
)

func main() {
	// gogpu/systray uses native UI message loops; pin the tray loop to the
	// process main thread for AppKit compatibility and stable native callbacks.
	runtime.LockOSThread()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
