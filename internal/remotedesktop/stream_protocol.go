package remotedesktop

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	rdpProtocolVersion byte = 1

	rdpOpBind byte = 0x01
	rdpOpAck  byte = 0x02
	rdpOpDrop byte = 0x03

	rdpOpReady  byte = 0x81
	rdpOpUpdate byte = 0x82
	rdpOpError  byte = 0x83

	rdpUpdateFlagKeyFrame byte = 0x01

	rdpUpdateHeaderBytes = 32
	rdpRectHeaderBytes   = 16
	rdpReadyPayloadBytes = 4 + 4 + 8 + 8

	rdpMaxBindIDSize    = 1024
	rdpMaxBindTokenSize = 128
	rdpMaxReasonSize    = 512

	rdpMaxRectsPerFrame = 4096

	rdpErrorProtocol  byte = 1
	rdpErrorBinding   byte = 2
	rdpErrorSession   byte = 3
	rdpErrorTransport byte = 4
)

var errInvalidRDPFrame = errors.New("invalid rdp stream frame")

type rdpBind struct {
	sessionID string
	token     string
}

type rdpAck struct {
	batchID      uint64
	renderMicros uint32
}

type rdpDrop struct {
	batchID uint64
	reason  byte
}

func encodeRDPBindFrame(sessionID, token string) ([]byte, error) {
	if sessionID == "" || len(sessionID) > rdpMaxBindIDSize {
		return nil, errInvalidRDPFrame
	}
	if token == "" || len(token) > rdpMaxBindTokenSize {
		return nil, errInvalidRDPFrame
	}
	frame := make([]byte, 6+len(sessionID)+len(token))
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpBind
	binary.LittleEndian.PutUint16(frame[2:4], uint16(len(sessionID)))
	binary.LittleEndian.PutUint16(frame[4:6], uint16(len(token)))
	copy(frame[6:], sessionID)
	copy(frame[6+len(sessionID):], token)
	return frame, nil
}

func parseRDPBindFrame(frame []byte) (rdpBind, error) {
	if len(frame) < 6 || frame[0] != rdpProtocolVersion || frame[1] != rdpOpBind {
		return rdpBind{}, errInvalidRDPFrame
	}
	idSize := int(binary.LittleEndian.Uint16(frame[2:4]))
	tokenSize := int(binary.LittleEndian.Uint16(frame[4:6]))
	if idSize < 1 || idSize > rdpMaxBindIDSize || tokenSize < 1 || tokenSize > rdpMaxBindTokenSize {
		return rdpBind{}, errInvalidRDPFrame
	}
	if len(frame) != 6+idSize+tokenSize {
		return rdpBind{}, errInvalidRDPFrame
	}
	id := frame[6 : 6+idSize]
	token := frame[6+idSize:]
	if !utf8.Valid(id) || !utf8.Valid(token) {
		return rdpBind{}, errInvalidRDPFrame
	}
	return rdpBind{sessionID: string(id), token: string(token)}, nil
}

func parseRDPControlFrame(frame []byte) (op byte, ack rdpAck, drop rdpDrop, err error) {
	if len(frame) < 2 || frame[0] != rdpProtocolVersion {
		return 0, rdpAck{}, rdpDrop{}, errInvalidRDPFrame
	}
	switch frame[1] {
	case rdpOpAck:
		if len(frame) != 15 {
			return 0, rdpAck{}, rdpDrop{}, errInvalidRDPFrame
		}
		return rdpOpAck, rdpAck{
			batchID:      binary.LittleEndian.Uint64(frame[2:10]),
			renderMicros: binary.LittleEndian.Uint32(frame[10:14]),
		}, rdpDrop{}, nil
	case rdpOpDrop:
		if len(frame) != 11 {
			return 0, rdpAck{}, rdpDrop{}, errInvalidRDPFrame
		}
		return rdpOpDrop, rdpAck{}, rdpDrop{
			batchID: binary.LittleEndian.Uint64(frame[2:10]),
			reason:  frame[10],
		}, nil
	default:
		return 0, rdpAck{}, rdpDrop{}, errInvalidRDPFrame
	}
}

func encodeRDPAckFrame(batchID uint64, renderMicros uint32) []byte {
	frame := make([]byte, 15)
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpAck
	binary.LittleEndian.PutUint64(frame[2:10], batchID)
	binary.LittleEndian.PutUint32(frame[10:14], renderMicros)
	return frame
}

func encodeRDPDropFrame(batchID uint64, reason byte) []byte {
	frame := make([]byte, 11)
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpDrop
	binary.LittleEndian.PutUint64(frame[2:10], batchID)
	frame[10] = reason
	return frame
}

func encodeRDPReadyFrame(width, height int, batchID uint64) []byte {
	frame := make([]byte, 2+rdpReadyPayloadBytes)
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpReady
	binary.LittleEndian.PutUint32(frame[2:6], uint32(width))
	binary.LittleEndian.PutUint32(frame[6:10], uint32(height))
	binary.LittleEndian.PutUint64(frame[10:18], batchID)
	return frame
}

func encodeRDPErrorFrame(code byte, reason string) []byte {
	value := []byte(reason)
	if len(value) > rdpMaxReasonSize {
		value = value[:rdpMaxReasonSize]
		for len(value) > 0 && !utf8.Valid(value) {
			value = value[:len(value)-1]
		}
	}
	frame := make([]byte, 3+len(value))
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpError
	frame[2] = code
	copy(frame[3:], value)
	return frame
}

func buildRDPUpdateFrame(batchID uint64, update *CanvasUpdate, rects []EncodedRect) ([]byte, error) {
	if update == nil {
		return nil, errInvalidRDPFrame
	}
	if len(rects) > rdpMaxRectsPerFrame {
		return nil, fmt.Errorf("单帧矩形过多: %d", len(rects))
	}
	total := rdpUpdateHeaderBytes + len(rects)*rdpRectHeaderBytes
	for _, rect := range rects {
		total += len(rect.Payload)
	}
	frame := make([]byte, total)
	frame[0] = rdpProtocolVersion
	frame[1] = rdpOpUpdate
	if update.KeyFrame {
		frame[2] |= rdpUpdateFlagKeyFrame
	}
	binary.LittleEndian.PutUint64(frame[4:12], batchID)
	binary.LittleEndian.PutUint64(frame[12:20], update.Version)
	binary.LittleEndian.PutUint32(frame[20:24], uint32(update.Width))
	binary.LittleEndian.PutUint32(frame[24:28], uint32(update.Height))
	binary.LittleEndian.PutUint16(frame[28:30], uint16(len(rects)))

	offset := rdpUpdateHeaderBytes
	payloadOffset := rdpUpdateHeaderBytes + len(rects)*rdpRectHeaderBytes
	for index, rect := range rects {
		descriptor := frame[offset : offset+rdpRectHeaderBytes]
		binary.LittleEndian.PutUint16(descriptor[0:2], uint16(rect.X))
		binary.LittleEndian.PutUint16(descriptor[2:4], uint16(rect.Y))
		binary.LittleEndian.PutUint16(descriptor[4:6], uint16(rect.Width))
		binary.LittleEndian.PutUint16(descriptor[6:8], uint16(rect.Height))
		descriptor[8] = byte(rect.Format)
		binary.LittleEndian.PutUint32(descriptor[12:16], uint32(len(rect.Payload)))
		copy(frame[payloadOffset:], rect.Payload)
		offset += rdpRectHeaderBytes
		payloadOffset += len(rect.Payload)
		_ = index
	}
	return frame, nil
}

type parsedUpdate struct {
	keyFrame      bool
	batchID       uint64
	sequence      uint64
	width, height int
	rects         []parsedRect
}

type parsedRect struct {
	rect       Rect
	format     updateFormat
	payload    []byte
	payloadLen int
}

func parseRDPUpdateFrame(frame []byte) (parsedUpdate, error) {
	if len(frame) < rdpUpdateHeaderBytes || frame[0] != rdpProtocolVersion || frame[1] != rdpOpUpdate {
		return parsedUpdate{}, errInvalidRDPFrame
	}
	result := parsedUpdate{
		keyFrame: frame[2]&rdpUpdateFlagKeyFrame != 0,
		batchID:  binary.LittleEndian.Uint64(frame[4:12]),
		sequence: binary.LittleEndian.Uint64(frame[12:20]),
		width:    int(binary.LittleEndian.Uint32(frame[20:24])),
		height:   int(binary.LittleEndian.Uint32(frame[24:28])),
	}
	count := int(binary.LittleEndian.Uint16(frame[28:30]))
	if count > rdpMaxRectsPerFrame {
		return parsedUpdate{}, errInvalidRDPFrame
	}
	descriptorOffset := rdpUpdateHeaderBytes
	payloadOffset := rdpUpdateHeaderBytes + count*rdpRectHeaderBytes
	if payloadOffset > len(frame) {
		return parsedUpdate{}, errInvalidRDPFrame
	}
	for index := 0; index < count; index++ {
		descriptor := frame[descriptorOffset : descriptorOffset+rdpRectHeaderBytes]
		rect := Rect{
			X:      int(binary.LittleEndian.Uint16(descriptor[0:2])),
			Y:      int(binary.LittleEndian.Uint16(descriptor[2:4])),
			Width:  int(binary.LittleEndian.Uint16(descriptor[4:6])),
			Height: int(binary.LittleEndian.Uint16(descriptor[6:8])),
		}
		format := updateFormat(descriptor[8])
		if format != updateFormatRGBA && format != updateFormatPNG {
			return parsedUpdate{}, errInvalidRDPFrame
		}
		length := int(binary.LittleEndian.Uint32(descriptor[12:16]))
		if length < 0 || payloadOffset+length > len(frame) {
			return parsedUpdate{}, errInvalidRDPFrame
		}
		result.rects = append(result.rects, parsedRect{
			rect:       rect,
			format:     format,
			payload:    frame[payloadOffset : payloadOffset+length],
			payloadLen: length,
		})
		descriptorOffset += rdpRectHeaderBytes
		payloadOffset += length
	}
	if payloadOffset != len(frame) {
		return parsedUpdate{}, errInvalidRDPFrame
	}
	return result, nil
}
