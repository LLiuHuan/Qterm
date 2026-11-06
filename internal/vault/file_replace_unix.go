//go:build !windows

package vault

import "os"

func replaceFileAtomically(source, target string) error {
	return os.Rename(source, target)
}

func protectLocalPath(string, bool) error { return nil }
