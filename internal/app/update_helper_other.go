//go:build !windows

package app

import (
	"errors"

	"changeme/internal/update"
)

func applyUpdatePlan(_ update.UpdatePlan) error {
	return errors.New("update helper mode is only supported on Windows")
}
