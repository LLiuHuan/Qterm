package remoteedit

import (
	"os/exec"
	"runtime"
	"strings"
)

type lookPathFunc func(string) (string, error)

func editorCommand(goos string, configured string, localPath string, lookPath lookPathFunc) (string, []string) {
	if trimmed := strings.TrimSpace(configured); trimmed != "" {

		if goos == "darwin" && isApplicationBundle(trimmed) {
			return "open", []string{"-a", trimmed, localPath}
		}
		return trimmed, []string{localPath}
	}
	switch goos {
	case "darwin":

		if isApplicationBundle(localPath) {
			return "open", []string{"-a", localPath}
		}
		return "open", []string{localPath}
	case "windows":

		return "rundll32", []string{"url.dll,FileProtocolHandler", localPath}
	default:

		candidates := []struct {
			name string
			args []string
		}{
			{"xdg-open", []string{localPath}},
			{"gio", []string{"open", localPath}},
			{"kde-open", []string{localPath}},
			{"gnome-open", []string{localPath}},
		}
		for _, candidate := range candidates {
			if lookPath == nil {
				return candidate.name, candidate.args
			}
			if _, err := lookPath(candidate.name); err == nil {
				return candidate.name, candidate.args
			}
		}

		return "xdg-open", []string{localPath}
	}
}

func isApplicationBundle(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimRight(path, "/")), ".app")
}

func startLocalEditor(editorPath string, localPath string) error {
	name, args := editorCommand(runtime.GOOS, editorPath, localPath, exec.LookPath)
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
