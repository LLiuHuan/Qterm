//go:build windows

package clitool

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	userEnvironmentKey = `Environment`

	hwndBroadcast = 0xFFFF

	wmSettingChange = 0x001B

	smtoAbortIfHung = 0x0002

	notifyTimeoutMS = 5000
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

func installEntry(binary, linkPath string) error {
	return copyExecutable(binary, linkPath)
}

func removeEntry(linkPath string) error {
	return os.Remove(linkPath)
}

func entryInstalled(binary, linkPath string) bool {
	if strings.TrimSpace(binary) == "" {
		return false
	}
	return sameFileContent(binary, linkPath)
}

func entryOwned(linkPath string) (owned bool, exists bool) {
	info, err := os.Lstat(linkPath)
	if err != nil {
		return false, !errors.Is(err, os.ErrNotExist)
	}
	if info.IsDir() {
		return false, true
	}
	return true, true
}

func managesSearchPath() bool { return true }

func ensureSearchPath(dir, _ string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("empty search path entry")
	}
	raw, valueType, err := userPathRaw()
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	if containsPathEntry(raw, dir) {
		return nil
	}
	next := raw
	if strings.TrimSpace(next) == "" {
		next = dir
	} else {
		next = strings.TrimRight(next, ";") + ";" + dir
	}
	return writeUserPath(next, valueType)
}

func removeSearchPath(dir, _ string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	raw, valueType, err := userPathRaw()
	if err != nil {

		return nil
	}
	parts := strings.Split(raw, ";")
	kept := make([]string, 0, len(parts))
	removed := false
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if pathEntryEquals(part, dir) {
			removed = true
			continue
		}
		kept = append(kept, part)
	}
	if !removed {
		return nil
	}
	return writeUserPath(strings.Join(kept, ";"), valueType)
}

func removeStaleEntries(string, string) error {
	return nil
}

func pathSearchesDir(dir, pathValue string) bool {
	return pathContains(pathValue, dir)
}

func shellPathFile(string, string) string { return "" }

func searchPathValue(fallback string) string {
	raw, _, err := userPathRaw()
	if err != nil {
		return fallback
	}
	return expandPathValue(raw)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func userPathRaw() (raw string, valueType uint32, err error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, userEnvironmentKey, registry.QUERY_VALUE)
	if err != nil {
		return "", 0, err
	}
	defer key.Close()

	return key.GetStringValue("Path")
}

func writeUserPath(value string, valueType uint32) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, userEnvironmentKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if valueType == registry.EXPAND_SZ {
		if err := key.SetExpandStringValue("Path", value); err != nil {
			return err
		}
	} else if err := key.SetStringValue("Path", value); err != nil {
		return err
	}
	notifyEnvironmentChange()
	return nil
}

func containsPathEntry(raw, dir string) bool {
	for _, entry := range strings.Split(raw, ";") {
		if pathEntryEquals(entry, dir) {
			return true
		}
	}
	return false
}

func pathEntryEquals(entry, dir string) bool {
	expanded := expandPathValue(normalizePathEntry(entry))
	if expanded == "" {
		return false
	}
	return samePath(expanded, dir)
}

func normalizePathEntry(entry string) string {
	return strings.Trim(strings.TrimSpace(entry), `"`)
}

func expandPathValue(raw string) string {
	if expanded, err := registry.ExpandString(raw); err == nil {
		return expanded
	}
	return raw
}

func notifyEnvironmentChange() {
	name, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	_, _, _ = procSendMessageTimeoutW.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(name)),
		uintptr(smtoAbortIfHung),
		uintptr(notifyTimeoutMS),
		uintptr(unsafe.Pointer(&result)),
	)
}

func copyExecutable(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	temp := target + ".tmp"
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func sameFileContent(a, b string) bool {
	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(b)
	if err != nil || infoA.Size() != infoB.Size() {
		return false
	}
	contentA, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	contentB, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(contentA, contentB)
}
