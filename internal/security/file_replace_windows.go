//go:build windows

package security

import (
	"os"

	"golang.org/x/sys/windows"
)

func replaceFileAtomically(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func syncSecurityDirectory(path string) error {

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return file.Close()
}
