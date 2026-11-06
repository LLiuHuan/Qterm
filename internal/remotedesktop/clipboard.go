package remotedesktop

import "strings"

const (
	ClipboardModeOff = "off"

	ClipboardModeBoth = "both"

	ClipboardModeLocalToRemote = "local-to-remote"

	ClipboardModeRemoteToLocal = "remote-to-local"

	ClipboardContentText = "text"

	ClipboardContentTextAndFiles = "text-and-files"
)

type ClipboardSettings struct {
	Mode    string
	Content string
}

func NormalizeClipboardSettings(settings ClipboardSettings) ClipboardSettings {
	mode := strings.TrimSpace(settings.Mode)
	switch mode {
	case ClipboardModeOff, ClipboardModeLocalToRemote, ClipboardModeRemoteToLocal:
	default:
		mode = ClipboardModeBoth
	}
	content := strings.TrimSpace(settings.Content)
	switch content {
	case ClipboardContentTextAndFiles:
	default:
		content = ClipboardContentText
	}
	return ClipboardSettings{Mode: mode, Content: content}
}

func (c ClipboardSettings) AllowsLocalToRemote() bool {
	mode := NormalizeClipboardSettings(c).Mode
	return mode == ClipboardModeBoth || mode == ClipboardModeLocalToRemote
}

func (c ClipboardSettings) AllowsRemoteToLocal() bool {
	mode := NormalizeClipboardSettings(c).Mode
	return mode == ClipboardModeBoth || mode == ClipboardModeRemoteToLocal
}

func helperClipboardFeatureMask(settings ClipboardSettings) uint32 {
	normalized := NormalizeClipboardSettings(settings)
	switch normalized.Mode {
	case ClipboardModeOff:

		return 0
	}
	var mask uint32
	if normalized.AllowsLocalToRemote() {
		mask |= clipboardFlagLocalToRemote
	}
	if normalized.AllowsRemoteToLocal() {
		mask |= clipboardFlagRemoteToLocal
	}
	return mask
}

const (
	clipboardFlagLocalToRemote       uint32 = 0x01
	clipboardFlagLocalToRemoteFiles  uint32 = 0x02
	clipboardFlagRemoteToLocal       uint32 = 0x10
	clipboardFlagRemoteToLocalFiles  uint32 = 0x20
	clipboardFileTransferImplemented        = false
)

func advertisedClipboardContent(settings ClipboardSettings) string {
	normalized := NormalizeClipboardSettings(settings)
	if clipboardFileTransferImplemented && normalized.Content == ClipboardContentTextAndFiles {
		return ClipboardContentTextAndFiles
	}
	return ClipboardContentText
}
