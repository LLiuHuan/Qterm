package terminal

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

const (
	terminalProtocolVersion byte = 1

	terminalOpBind   byte = 0x01
	terminalOpInput  byte = 0x02
	terminalOpResize byte = 0x03
	terminalOpAck    byte = 0x04
	terminalOpClose  byte = 0x05

	terminalOpZmodem byte = 0x06

	terminalOpReady  byte = 0x81
	terminalOpOutput byte = 0x82
	terminalOpError  byte = 0x83

	terminalOpZmodemOutput byte = 0x84

	terminalOpZmodemDetected byte = 0x85

	terminalOpZmodemAck byte = 0x86

	terminalMaxDataSize   = 32 << 10
	terminalMaxReasonSize = 512
	terminalMaxTokenSize  = 128
	terminalMaxIDSize     = 1024
)

var errInvalidTerminalFrame = errors.New("invalid terminal protocol frame")

type terminalBind struct {
	sessionID string
	token     string
	columns   int
	rows      int
}

func terminalFrame(op byte, payload []byte) ([]byte, error) {
	if !terminalServerOpcode(op) || !terminalPayloadSizeValid(op, len(payload)) {
		return nil, errInvalidTerminalFrame
	}
	frame := make([]byte, 2+len(payload))
	frame[0] = terminalProtocolVersion
	frame[1] = op
	copy(frame[2:], payload)
	return frame, nil
}

func parseTerminalFrame(frame []byte) (byte, []byte, error) {
	if len(frame) < 2 || frame[0] != terminalProtocolVersion {
		return 0, nil, errInvalidTerminalFrame
	}
	op := frame[1]
	payload := frame[2:]
	if !terminalClientOpcode(op) || !terminalPayloadSizeValid(op, len(payload)) {
		return 0, nil, errInvalidTerminalFrame
	}
	return op, payload, nil
}

func parseTerminalBind(payload []byte) (terminalBind, error) {
	const fixedSize = 8
	if len(payload) < fixedSize {
		return terminalBind{}, errInvalidTerminalFrame
	}
	columns := int(binary.BigEndian.Uint16(payload[0:2]))
	rows := int(binary.BigEndian.Uint16(payload[2:4]))
	idSize := int(binary.BigEndian.Uint16(payload[4:6]))
	tokenSize := int(binary.BigEndian.Uint16(payload[6:8]))
	if columns < 1 || rows < 1 || idSize < 1 || idSize > terminalMaxIDSize ||
		tokenSize < 1 || tokenSize > terminalMaxTokenSize || len(payload) != fixedSize+idSize+tokenSize {
		return terminalBind{}, errInvalidTerminalFrame
	}
	id := payload[fixedSize : fixedSize+idSize]
	token := payload[fixedSize+idSize:]
	if !utf8.Valid(id) || !utf8.Valid(token) {
		return terminalBind{}, errInvalidTerminalFrame
	}
	return terminalBind{
		sessionID: string(id),
		token:     string(token),
		columns:   columns,
		rows:      rows,
	}, nil
}

func parseTerminalResize(payload []byte) (int, int, error) {
	if len(payload) != 4 {
		return 0, 0, errInvalidTerminalFrame
	}
	columns := int(binary.BigEndian.Uint16(payload[0:2]))
	rows := int(binary.BigEndian.Uint16(payload[2:4]))
	if columns < 1 || rows < 1 {
		return 0, 0, errInvalidTerminalFrame
	}
	return columns, rows, nil
}

func parseTerminalAck(payload []byte) (uint32, error) {
	if len(payload) != 4 {
		return 0, errInvalidTerminalFrame
	}
	value := binary.BigEndian.Uint32(payload)
	if value == 0 {
		return 0, errInvalidTerminalFrame
	}
	return value, nil
}

func terminalErrorFrame(code byte, reason string) []byte {
	value := []byte(reason)
	if len(value) > terminalMaxReasonSize {
		value = value[:terminalMaxReasonSize]
		for len(value) > 0 && !utf8.Valid(value) {
			value = value[:len(value)-1]
		}
	}
	payload := make([]byte, 1+len(value))
	payload[0] = code
	copy(payload[1:], value)
	frame, _ := terminalFrame(terminalOpError, payload)
	return frame
}

func terminalClientOpcode(op byte) bool {
	return op == terminalOpBind || op == terminalOpInput || op == terminalOpResize ||
		op == terminalOpAck || op == terminalOpClose || op == terminalOpZmodem
}

func terminalServerOpcode(op byte) bool {
	return op == terminalOpReady || op == terminalOpOutput || op == terminalOpClose ||
		op == terminalOpError || op == terminalOpZmodemOutput || op == terminalOpZmodemDetected || op == terminalOpZmodemAck
}

func terminalPayloadSizeValid(op byte, size int) bool {
	switch op {
	case terminalOpBind:
		return size >= 8 && size <= 8+terminalMaxIDSize+terminalMaxTokenSize
	case terminalOpZmodem, terminalOpInput, terminalOpOutput, terminalOpZmodemOutput:
		return size > 0 && size <= terminalMaxDataSize
	case terminalOpResize, terminalOpAck, terminalOpZmodemAck:
		return size == 4
	case terminalOpClose:
		return size <= terminalMaxReasonSize
	case terminalOpReady:
		return size == 0
	case terminalOpError:
		return size >= 1 && size <= 1+terminalMaxReasonSize
	case terminalOpZmodemDetected:
		return size == 1
	default:
		return false
	}
}
