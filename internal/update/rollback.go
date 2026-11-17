package update

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const pendingUpdateFile = ".qterm-update-pending.json"

type pendingUpdate struct {
	Target string `json:"target"`

	Previous string `json:"previous"`

	Boots int `json:"boots"`
}

func pendingUpdatePath(executable string) string {
	return filepath.Join(filepath.Dir(executable), pendingUpdateFile)
}

func RecordPendingUpdate(executable string) error {
	marker := pendingUpdate{
		Target:   executable,
		Previous: executable + ".old",
		Boots:    0,
	}
	body, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pendingUpdatePath(executable), body, 0o600)
}

func readPendingUpdate(executable string) (pendingUpdate, string, bool) {
	path := pendingUpdatePath(executable)
	body, err := os.ReadFile(path)
	if err != nil {
		return pendingUpdate{}, path, false
	}
	var marker pendingUpdate
	if err := json.Unmarshal(body, &marker); err != nil {

		_ = os.Remove(path)
		return pendingUpdate{}, path, false
	}
	if marker.Target == "" || !samePath(marker.Target, executable) {

		_ = os.Remove(path)
		return pendingUpdate{}, path, false
	}
	return marker, path, true
}

func ConfirmStartup(executable string) {
	marker, path, ok := readPendingUpdate(executable)
	if !ok {
		return
	}
	_ = os.Remove(path)

	if marker.Previous != "" {
		_ = os.Remove(marker.Previous)
	}
}

func TryRollbackAfterFailedUpdate(executable string, relaunch func(string) error) bool {
	marker, path, ok := readPendingUpdate(executable)
	if !ok {
		return false
	}
	if marker.Boots < 1 {

		marker.Boots++
		if body, err := json.MarshalIndent(marker, "", "  "); err == nil {
			_ = os.WriteFile(path, body, 0o600)
		}
		return false
	}

	if _, err := os.Stat(marker.Previous); err != nil {

		_ = os.Remove(path)
		return false
	}
	failed := executable + ".failed"
	_ = os.Remove(failed)
	if err := os.Rename(executable, failed); err != nil {

		_ = os.Remove(path)
		return false
	}
	if err := os.Rename(marker.Previous, executable); err != nil {

		_ = os.Rename(failed, executable)
		_ = os.Remove(path)
		return false
	}
	_ = os.Remove(path)
	if relaunch == nil {
		return false
	}
	if err := relaunch(executable); err != nil {
		return false
	}
	return true
}
