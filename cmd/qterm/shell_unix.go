//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyShellResize(target chan<- os.Signal) {
	signal.Notify(target, syscall.SIGWINCH)
}

func stopShellResize(target chan<- os.Signal) {
	signal.Stop(target)
}
