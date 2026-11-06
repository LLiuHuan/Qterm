//go:build !windows

package vault

import (
	"errors"
	"os"
)

func syncPasswordKeyDirectory(path string) (resultErr error) {
	directory, err := os.Open(path)
	if err != nil {
		return passwordKeyStoreError("open wrapped master-key directory for sync", err)
	}
	defer func() {
		if err := directory.Close(); err != nil {
			resultErr = errors.Join(
				resultErr,
				passwordKeyStoreError("close wrapped master-key directory after sync", err),
			)
		}
	}()
	if err := directory.Sync(); err != nil {
		return passwordKeyStoreError("sync wrapped master-key directory", err)
	}
	return nil
}
