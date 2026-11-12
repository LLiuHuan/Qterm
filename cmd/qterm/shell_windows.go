//go:build windows

package main

import "os"

func notifyShellResize(chan<- os.Signal) {}

func stopShellResize(chan<- os.Signal) {}
