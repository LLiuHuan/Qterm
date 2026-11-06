package vnc

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	bindTimeout = 10 * time.Second

	readBufferBytes = 128 * 1024
)

func ServeStream(stream Stream, resolver SessionResolver, tokens TokenVerifier) {
	if stream == nil {
		return
	}

	var closeOnce sync.Once
	closeStream := func() { closeOnce.Do(func() { _ = stream.Close() }) }
	defer closeStream()
	ctx := stream.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	frame, err := receiveWithTimeout(stream, ctx, bindTimeout)
	if err != nil {
		return
	}
	request, err := parseBindFrame(frame)
	if err != nil {
		_ = stream.Send(encodeErrorFrame(vncErrorProtocol, "绑定帧格式不正确"))
		return
	}
	if tokens == nil || !tokens.Consume(request.sessionID, request.token) {

		_ = stream.Send(encodeErrorFrame(vncErrorBinding, "绑定票据无效或已使用"))
		return
	}
	if resolver == nil {
		_ = stream.Send(encodeErrorFrame(vncErrorSession, "会话不可用"))
		return
	}
	session, found := resolver.StreamSession(request.sessionID)
	if !found {
		_ = stream.Send(encodeErrorFrame(vncErrorSession, "会话不存在或已结束"))
		return
	}
	if !session.Claim() {
		_ = stream.Send(encodeErrorFrame(vncErrorBinding, "会话已被其他窗口接管"))
		return
	}

	defer func() {
		if closer, ok := resolver.(sessionCloser); ok {
			_ = closer.Close(request.sessionID)
			return
		}
		_ = session.Close("画面流已结束")
	}()

	if err := stream.Send(encodeReadyFrame()); err != nil {
		return
	}

	uploadDone := make(chan struct{})
	go func() {
		defer close(uploadDone)
		for {
			payload, receiveErr := stream.Receive()
			if receiveErr != nil {
				_ = session.Close("前端已断开")
				return
			}
			if len(payload) == 0 {
				continue
			}
			if writeErr := session.Write(payload); writeErr != nil {
				return
			}
		}
	}()

	buffer := make([]byte, readBufferBytes)
	for {
		count, readErr := session.Read(buffer)
		if count > 0 {
			payload := make([]byte, count)
			copy(payload, buffer[:count])
			if sendErr := stream.Send(payload); sendErr != nil {
				_ = session.Close("前端已断开")
				break
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, ErrSessionNotFound) {
				_ = stream.Send(encodeErrorFrame(vncErrorTransport, "与服务端的连接已断开"))
			}
			break
		}
	}
	_ = session.Close("画面流已结束")

	closeStream()
	<-uploadDone
}

type sessionCloser interface {
	Close(id string) error
}

func receiveWithTimeout(stream Stream, ctx context.Context, timeout time.Duration) ([]byte, error) {
	type result struct {
		payload []byte
		err     error
	}
	channel := make(chan result, 1)
	go func() {
		payload, err := stream.Receive()
		channel <- result{payload: payload, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case received := <-channel:
		return received.payload, received.err
	case <-timer.C:
		_ = stream.Close()
		return nil, errors.New("bind timeout")
	case <-ctx.Done():
		_ = stream.Close()
		return nil, errors.New("bind cancelled")
	}
}
