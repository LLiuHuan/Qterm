//go:build !windows

package remotedesktop

import "os/exec"

func hideHelperProcessWindow(*exec.Cmd) {}
