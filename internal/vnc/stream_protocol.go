package vnc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	vncProtocolVersion byte = 1

	vncOpBind  byte = 0x01
	vncOpReady byte = 0x81
	vncOpError byte = 0x83

	vncReadyPayloadBytes = 6

	vncMaxBindIDSize    = 1024
	vncMaxBindTokenSize = 128
	vncMaxReasonSize    = 512

	vncErrorProtocol  byte = 1
	vncErrorBinding   byte = 2
	vncErrorSession   byte = 3
	vncErrorTransport byte = 4
)

var errInvalidFrame = errors.New("invalid vnc stream frame")

type bindRequest struct {
	sessionID string
	token     string
}

func encodeBindFrame(sessionID, token string) ([]byte, error) {
	if len(sessionID) > vncMaxBindIDSize {
		return nil, fmt.Errorf("vnc session id too long: %d", len(sessionID))
	}
	if len(token) > vncMaxBindTokenSize {
		return nil, fmt.Errorf("vnc stream token too long: %d", len(token))
	}
	frame := make([]byte, 0, 6+len(sessionID)+len(token))
	frame = append(frame, vncProtocolVersion, vncOpBind)
	frame = binary.LittleEndian.AppendUint16(frame, uint16(len(sessionID)))
	frame = binary.LittleEndian.AppendUint16(frame, uint16(len(token)))
	frame = append(frame, sessionID...)
	frame = append(frame, token...)
	return frame, nil
}

func parseBindFrame(frame []byte) (bindRequest, error) {
	if len(frame) < 6 {
		return bindRequest{}, fmt.Errorf("%w: bind header too short: %d", errInvalidFrame, len(frame))
	}
	if frame[0] != vncProtocolVersion {
		return bindRequest{}, fmt.Errorf("%w: unsupported version %d", errInvalidFrame, frame[0])
	}
	if frame[1] != vncOpBind {
		return bindRequest{}, fmt.Errorf("%w: unexpected op 0x%02x", errInvalidFrame, frame[1])
	}
	idLength := int(binary.LittleEndian.Uint16(frame[2:4]))
	tokenLength := int(binary.LittleEndian.Uint16(frame[4:6]))
	if idLength > vncMaxBindIDSize || tokenLength > vncMaxBindTokenSize {
		return bindRequest{}, fmt.Errorf("%w: bind lengths out of range (%d/%d)", errInvalidFrame, idLength, tokenLength)
	}
	if len(frame) != 6+idLength+tokenLength {
		return bindRequest{}, fmt.Errorf("%w: bind length mismatch: %d != %d", errInvalidFrame, len(frame), 6+idLength+tokenLength)
	}
	return bindRequest{
		sessionID: string(frame[6 : 6+idLength]),
		token:     string(frame[6+idLength:]),
	}, nil
}

func encodeReadyFrame() []byte {
	frame := make([]byte, 0, 2+vncReadyPayloadBytes)
	frame = append(frame, vncProtocolVersion, vncOpReady)
	return append(frame, make([]byte, vncReadyPayloadBytes)...)
}

func encodeErrorFrame(code byte, reason string) []byte {
	if len(reason) > vncMaxReasonSize {
		reason = reason[:vncMaxReasonSize]
		for len(reason) > 0 && !utf8.ValidString(reason) {
			reason = reason[:len(reason)-1]
		}
	}
	frame := make([]byte, 0, 3+len(reason))
	frame = append(frame, vncProtocolVersion, vncOpError, code)
	return append(frame, reason...)
}
