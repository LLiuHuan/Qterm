//go:build !windows

package security

import "os"

func replaceFileAtomically(source, target string) error {
	return os.Rename(source, target)
}

func syncSecurityDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
