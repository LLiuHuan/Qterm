package remotedesktop

import "time"

type SessionOptions struct {
	Host     string
	Port     int
	Username string
	Password string
	Domain   string
	Width    uint16
	Height   uint16

	CertificateFingerprint string

	DynamicResolutionDisabled bool

	FirstFrameTimeout time.Duration

	Clipboard ClipboardSettings
}

type MouseButton uint8

const (
	MouseLeft MouseButton = iota
	MouseRight
	MouseMiddle

	MouseX1
	MouseX2
)

type EventStats struct {
	Packets        int
	SlowPathPDUs   int
	BitmapUpdates  int
	OrdersUpdates  int
	PointerUpdates int
	OtherUpdates   int
	ChannelPackets int
	BitmapsApplied int
	BitmapsFailed  int
	OrdersApplied  int
	OrdersFailed   int
	PointerApplied int
	PointerFailed  int
}

type ClipboardStats struct {
	Text string

	Sequence uint64
}
