//go:build windows

package remotedesktop

func decodeHelperStderrLine(raw []byte) string {
	return decodeHelperConsoleText(raw)
}
