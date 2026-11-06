//go:build !darwin || !cgo

package hostimport

import "errors"

func darwinKeychainPassword(service, account string) (string, error) {
	return "", errors.New("native keychain unavailable")
}

var errKeychainItemNotFound = errors.New("keychain item not found")
