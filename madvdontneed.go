// +build linux

package main

import (
	"log"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

// Inject madvdontneed=1 as soon as possible.

var _ = func() struct{} {
	// If GODEBUG is not empty, then use that instead of trying to exec our own.
	if os.Getenv("GODEBUG") != "" {
		return struct{}{}
	}

	// Do magic.
	var environ = append(os.Environ(), "GODEBUG=madvdontneed=1")

	log.Println("execve(2)ing with madvdontneed=1 for aggressive GC.")

	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		return struct{}{}
	}

	if err := syscall.Exec(path, os.Args, environ); err != nil {
		log.Println("Error while executing:", err)
		log.Println("Starting up without madvdontneed=1...")
		return struct{}{}
	}

	os.Exit(0)
	return struct{}{}
}()

func init() {
	// Aggressive memory freeing you asked, so aggressive memory freeing we will
	// deliver.
	if strings.Contains(os.Getenv("GODEBUG"), "madvdontneed=1") {
		go func() {
			log.Println("Now attempting to free memory every 5s... (madvdontneed=1)")
			for range time.Tick(5 * time.Second) {
				debug.FreeOSMemory()
			}
		}()
	}
}
