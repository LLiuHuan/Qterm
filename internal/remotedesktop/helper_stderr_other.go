//go:build !windows

package remotedesktop

func decodeHelperStderrLine(raw []byte) string {
	return string(raw)
}
